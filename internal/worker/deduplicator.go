package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type Deduplicator interface {
	TryBegin(ctx context.Context, eventID string) (bool, error)
	MarkDone(ctx context.Context, eventID string) error
	ClearInFlight(ctx context.Context, eventID string) error
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

func (d *RedisDeduplicator) TryBegin(ctx context.Context, eventID string) (bool, error) {
	if eventID == "" {
		return true, nil
	}
	res, err := d.cmd.Eval(ctx, redisDeduperTryBeginScript,
		[]string{d.doneKey(eventID), d.inFlightKey(eventID)},
		d.inFlightTTL.Milliseconds()).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

func (d *RedisDeduplicator) MarkDone(ctx context.Context, eventID string) error {
	if eventID == "" {
		return nil
	}
	return d.cmd.Eval(ctx, redisDeduperMarkDoneScript,
		[]string{d.doneKey(eventID), d.inFlightKey(eventID)},
		d.doneTTL.Milliseconds()).Err()
}

func (d *RedisDeduplicator) ClearInFlight(ctx context.Context, eventID string) error {
	if eventID == "" {
		return nil
	}
	return d.cmd.Del(ctx, d.inFlightKey(eventID)).Err()
}

func (d *RedisDeduplicator) doneKey(eventID string) string {
	return fmt.Sprintf("consumer:event:done:%s:%s", d.namespace, eventID)
}

func (d *RedisDeduplicator) inFlightKey(eventID string) string {
	return fmt.Sprintf("consumer:event:processing:%s:%s", d.namespace, eventID)
}

const redisDeduperTryBeginScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
	return 0
end
if redis.call('SET', KEYS[2], '1', 'NX', 'PX', tonumber(ARGV[1])) then
	return 1
end
return 0
`

const redisDeduperMarkDoneScript = `
redis.call('SET', KEYS[1], '1', 'PX', tonumber(ARGV[1]))
redis.call('DEL', KEYS[2])
return 1
`
