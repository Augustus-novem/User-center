package worker

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisDeduplicator_OwnerCheckedStateMachine(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	deduper := NewRedisDeduplicator(rdb, "test:"+uuid.NewString())
	eventID := uuid.NewString()
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), deduper.doneKey(eventID), deduper.inFlightKey(eventID)).Err()
	})

	first, err := deduper.TryBegin(ctx, eventID)
	if err != nil || first.State != DeduplicationAcquired || first.OwnerToken == "" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	busy, err := deduper.TryBegin(ctx, eventID)
	if err != nil || busy.State != DeduplicationBusy {
		t.Fatalf("busy=%+v err=%v", busy, err)
	}
	if err = deduper.MarkDone(ctx, eventID, "not-the-owner"); !errors.Is(err, ErrDedupOwnershipLost) {
		t.Fatalf("wrong owner MarkDone error=%v", err)
	}
	if exists, _ := rdb.Exists(ctx, deduper.doneKey(eventID)).Result(); exists != 0 {
		t.Fatal("wrong owner must not create done key")
	}
	if err = deduper.MarkDone(ctx, eventID, first.OwnerToken); err != nil {
		t.Fatalf("owner MarkDone: %v", err)
	}
	done, err := deduper.TryBegin(ctx, eventID)
	if err != nil || done.State != DeduplicationDone {
		t.Fatalf("done=%+v err=%v", done, err)
	}
}

func TestRedisDeduplicator_StaleOwnerCannotClearNewLease(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })

	deduper := NewRedisDeduplicator(rdb, "test:"+uuid.NewString())
	eventID := uuid.NewString()
	t.Cleanup(func() { _ = rdb.Del(context.Background(), deduper.inFlightKey(eventID)).Err() })
	stale, err := deduper.TryBegin(ctx, eventID)
	if err != nil {
		t.Fatal(err)
	}
	const newOwner = "new-owner-token"
	if err = rdb.Set(ctx, deduper.inFlightKey(eventID), newOwner, time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if err = deduper.ClearInFlight(ctx, eventID, stale.OwnerToken); !errors.Is(err, ErrDedupOwnershipLost) {
		t.Fatalf("stale clear error=%v", err)
	}
	if got, err := rdb.Get(ctx, deduper.inFlightKey(eventID)).Result(); err != nil || got != newOwner {
		t.Fatalf("new lease was changed: got=%q err=%v", got, err)
	}
}

func TestRunDeduplicated_CleanupUsesIndependentBoundedContext(t *testing.T) {
	original, cancelOriginal := context.WithCancel(context.Background())
	deduper := &cleanupContextDeduplicator{}
	_, err := RunDeduplicated(original, deduper, "evt", func(context.Context) error {
		cancelOriginal()
		return errors.New("business failed")
	})
	if err == nil {
		t.Fatal("want business error")
	}
	if deduper.cleanupErr != nil {
		t.Fatalf("cleanup context should survive session cancellation: %v", deduper.cleanupErr)
	}
	if !deduper.hadDeadline {
		t.Fatal("cleanup context must be bounded by a deadline")
	}
}

type cleanupContextDeduplicator struct {
	cleanupErr  error
	hadDeadline bool
}

func (*cleanupContextDeduplicator) TryBegin(context.Context, string) (DeduplicationLease, error) {
	return DeduplicationLease{State: DeduplicationAcquired, OwnerToken: "owner"}, nil
}
func (*cleanupContextDeduplicator) MarkDone(context.Context, string, string) error { return nil }
func (d *cleanupContextDeduplicator) ClearInFlight(ctx context.Context, _, _ string) error {
	d.cleanupErr = ctx.Err()
	_, d.hadDeadline = ctx.Deadline()
	return nil
}
