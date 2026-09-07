package cache

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

type hotRankCmdableStub struct {
	redis.Cmdable
	evalFn   func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
	existsFn func(ctx context.Context, keys ...string) *redis.IntCmd
	zrevFn   func(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd
}

func (s *hotRankCmdableStub) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	return s.existsFn(ctx, keys...)
}

func (s *hotRankCmdableStub) ZRevRangeWithScores(ctx context.Context, key string, start, stop int64) *redis.ZSliceCmd {
	return s.zrevFn(ctx, key, start, stop)
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

func TestRedisHotRankCache_CreateSnapshotAndPage(t *testing.T) {
	t.Parallel()
	snapshot := time.Date(2026, 9, 7, 20, 10, 0, 0, time.Local)
	var evalKeys []string
	var rangeKey string
	var start, stop int64
	c := NewRedisHotRankCache(&hotRankCmdableStub{
		evalFn: func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
			evalKeys = keys
			return redis.NewCmdResult(int64(1), nil)
		},
		zrevFn: func(ctx context.Context, key string, gotStart, gotStop int64) *redis.ZSliceCmd {
			rangeKey, start, stop = key, gotStart, gotStop
			return redis.NewZSliceCmdResult([]redis.Z{{Member: "9", Score: 18}, {Member: "7", Score: 12}}, nil)
		},
	}, 3*time.Minute, 10*time.Minute, 24*time.Hour)

	items, err := c.SnapshotPage(context.Background(), snapshot, 2, 2, true)
	if err != nil {
		t.Fatal(err)
	}
	minute := snapshot.Unix() / 60
	wantKeys := []string{
		fmt.Sprintf("hot:note:snapshot:%d", minute),
		fmt.Sprintf("hot:note:snapshot:ready:%d", minute),
		"hot:note:202609072008",
		"hot:note:202609072009",
		"hot:note:202609072010",
	}
	if !reflect.DeepEqual(evalKeys, wantKeys) {
		t.Fatalf("snapshot keys=%v", evalKeys)
	}
	if rangeKey != wantKeys[0] || start != 2 || stop != 3 || len(items) != 2 || items[0].NoteID != 9 || items[0].Score != 18 {
		t.Fatalf("range=%s %d..%d items=%+v", rangeKey, start, stop, items)
	}
}

func TestRedisHotRankCache_ExpiredSnapshot(t *testing.T) {
	t.Parallel()
	c := NewRedisHotRankCache(&hotRankCmdableStub{existsFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
		return redis.NewIntResult(0, nil)
	}}, time.Hour, 10*time.Minute, 24*time.Hour)
	_, err := c.SnapshotPage(context.Background(), time.Now(), 0, 10, false)
	if !errors.Is(err, ErrHotRankSnapshotExpired) {
		t.Fatalf("want ErrHotRankSnapshotExpired, got %v", err)
	}
}
