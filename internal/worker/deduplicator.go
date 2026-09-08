package worker

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type DeduplicationState string

const (
	DeduplicationAcquired DeduplicationState = "ACQUIRED"
	DeduplicationDone     DeduplicationState = "DONE"
	DeduplicationBusy     DeduplicationState = "BUSY"
	cleanupTimeout                           = 2 * time.Second
)

var (
	ErrMessageInFlight    = errors.New("message is already in flight")
	ErrDedupOwnershipLost = errors.New("deduplication lease ownership lost")
)

type DeduplicationLease struct {
	State      DeduplicationState
	OwnerToken string
}

type Deduplicator interface {
	TryBegin(ctx context.Context, eventID string) (DeduplicationLease, error)
	MarkDone(ctx context.Context, eventID, ownerToken string) error
	ClearInFlight(ctx context.Context, eventID, ownerToken string) error
}

type RedisDeduplicator struct {
	cmd         redis.Cmdable
	doneTTL     time.Duration
	inFlightTTL time.Duration
	namespace   string
}

func NewRedisDeduplicator(cmd redis.Cmdable, namespace ...string) *RedisDeduplicator {
	ns := "default"
	if len(namespace) > 0 && strings.TrimSpace(namespace[0]) != "" {
		ns = strings.TrimSpace(namespace[0])
	}
	return &RedisDeduplicator{
		cmd:         cmd,
		doneTTL:     7 * 24 * time.Hour,
		inFlightTTL: 5 * time.Minute,
		namespace:   ns,
	}
}

func (d *RedisDeduplicator) TryBegin(ctx context.Context, eventID string) (DeduplicationLease, error) {
	if strings.TrimSpace(eventID) == "" {
		return DeduplicationLease{}, errors.New("event id is empty")
	}
	ownerToken, err := newOwnerToken()
	if err != nil {
		return DeduplicationLease{}, fmt.Errorf("generate deduplication owner token: %w", err)
	}
	res, err := d.cmd.Eval(ctx, redisDeduperTryBeginScript,
		[]string{d.doneKey(eventID), d.inFlightKey(eventID)},
		ownerToken, d.inFlightTTL.Milliseconds()).Int()
	if err != nil {
		return DeduplicationLease{}, fmt.Errorf("acquire deduplication lease: %w", err)
	}
	switch res {
	case 1:
		return DeduplicationLease{State: DeduplicationAcquired, OwnerToken: ownerToken}, nil
	case 2:
		return DeduplicationLease{State: DeduplicationDone}, nil
	case 3:
		return DeduplicationLease{State: DeduplicationBusy}, nil
	default:
		return DeduplicationLease{}, fmt.Errorf("unexpected deduplication state %d", res)
	}
}

func (d *RedisDeduplicator) MarkDone(ctx context.Context, eventID, ownerToken string) error {
	if strings.TrimSpace(eventID) == "" || strings.TrimSpace(ownerToken) == "" {
		return ErrDedupOwnershipLost
	}
	res, err := d.cmd.Eval(ctx, redisDeduperMarkDoneScript,
		[]string{d.doneKey(eventID), d.inFlightKey(eventID)},
		ownerToken, d.doneTTL.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("mark deduplication done: %w", err)
	}
	if res != 1 {
		return ErrDedupOwnershipLost
	}
	return nil
}

func (d *RedisDeduplicator) ClearInFlight(ctx context.Context, eventID, ownerToken string) error {
	if strings.TrimSpace(eventID) == "" || strings.TrimSpace(ownerToken) == "" {
		return ErrDedupOwnershipLost
	}
	res, err := d.cmd.Eval(ctx, redisDeduperClearScript,
		[]string{d.inFlightKey(eventID)}, ownerToken).Int()
	if err != nil {
		return fmt.Errorf("clear deduplication lease: %w", err)
	}
	if res != 1 {
		return ErrDedupOwnershipLost
	}
	return nil
}

func RunDeduplicated(
	ctx context.Context,
	deduper Deduplicator,
	eventID string,
	business func(context.Context) error,
) (DeduplicationState, error) {
	lease, err := deduper.TryBegin(ctx, eventID)
	if err != nil {
		return "", err
	}
	switch lease.State {
	case DeduplicationDone:
		return DeduplicationDone, nil
	case DeduplicationBusy:
		return DeduplicationBusy, ErrMessageInFlight
	case DeduplicationAcquired:
	default:
		return "", fmt.Errorf("unexpected deduplication state %q", lease.State)
	}

	if err = business(ctx); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cleanupTimeout)
		cleanupErr := deduper.ClearInFlight(cleanupCtx, eventID, lease.OwnerToken)
		cancel()
		if cleanupErr != nil {
			return DeduplicationAcquired, errors.Join(err, fmt.Errorf("cleanup failed lease: %w", cleanupErr))
		}
		return DeduplicationAcquired, err
	}
	if err = deduper.MarkDone(ctx, eventID, lease.OwnerToken); err != nil {
		return DeduplicationAcquired, err
	}
	return DeduplicationAcquired, nil
}

func newOwnerToken() (string, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw[:]), nil
}

func (d *RedisDeduplicator) doneKey(eventID string) string {
	return fmt.Sprintf("consumer:event:done:%s:%s", d.namespace, eventID)
}

func (d *RedisDeduplicator) inFlightKey(eventID string) string {
	return fmt.Sprintf("consumer:event:processing:%s:%s", d.namespace, eventID)
}

const redisDeduperTryBeginScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
	return 2
end
if redis.call('SET', KEYS[2], ARGV[1], 'NX', 'PX', tonumber(ARGV[2])) then
	return 1
end
return 3
`

const redisDeduperMarkDoneScript = `
if redis.call('GET', KEYS[2]) ~= ARGV[1] then
	return 0
end
redis.call('SET', KEYS[1], '1', 'PX', tonumber(ARGV[2]))
redis.call('DEL', KEYS[2])
return 1
`

const redisDeduperClearScript = `
if redis.call('GET', KEYS[1]) ~= ARGV[1] then
	return 0
end
redis.call('DEL', KEYS[1])
return 1
`
