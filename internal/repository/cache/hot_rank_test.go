package cache

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type hotRankCmdableStub struct {
	redis.Cmdable
	evalFn func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
}

func (s *hotRankCmdableStub) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	return s.evalFn(ctx, script, keys, args...)
}

func TestRedisHotRankCache_RecordBucketBoundary(t *testing.T) {
	t.Parallel()
	var keys []string
	var args []any
	cmd := &hotRankCmdableStub{evalFn: func(ctx context.Context, script string, gotKeys []string, gotArgs ...any) *redis.Cmd {
		keys = gotKeys
		args = gotArgs
		return redis.NewCmdResult(int64(1), nil)
	}}
	c := NewRedisHotRankCache(cmd, 60*time.Minute, 10*time.Minute, 7*24*time.Hour)
	occurredAt := time.Date(2026, 9, 7, 20, 10, 59, 999_000_000, time.Local)

	applied, err := c.Record(context.Background(), "evt-1", 42, occurredAt, 3)
	if err != nil || !applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	if !reflect.DeepEqual(keys, []string{"hot:note:202609072010", "hot:event:done:evt-1"}) {
		t.Fatalf("keys=%v", keys)
	}
	wantExpiry := occurredAt.Truncate(time.Minute).Add(time.Minute + 70*time.Minute).UnixMilli()
	if args[0] != int64(3) || args[1] != "42" || args[2] != wantExpiry || args[3] != int64((7*24*time.Hour)/time.Millisecond) {
		t.Fatalf("args=%v", args)
	}
}

func TestRedisHotRankCache_RecordDuplicate(t *testing.T) {
	t.Parallel()
	c := NewRedisHotRankCache(&hotRankCmdableStub{evalFn: func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
		return redis.NewCmdResult(int64(0), nil)
	}}, time.Hour, 10*time.Minute, 24*time.Hour)
	applied, err := c.Record(context.Background(), "duplicate", 1, time.Now(), 10)
	if err != nil || applied {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
}

func TestRedisHotRankCache_ExpiredBucketUsesPastExpiry(t *testing.T) {
	t.Parallel()
	var expireAt int64
	c := NewRedisHotRankCache(&hotRankCmdableStub{evalFn: func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
		expireAt = args[2].(int64)
		return redis.NewCmdResult(int64(1), nil)
	}}, time.Hour, 10*time.Minute, 24*time.Hour)
	old := time.Now().Add(-2 * time.Hour)
	if _, err := c.Record(context.Background(), "old", 1, old, 10); err != nil {
		t.Fatal(err)
	}
	if expireAt >= time.Now().UnixMilli() {
		t.Fatalf("expired event must not revive its bucket: expireAt=%d", expireAt)
	}
}
