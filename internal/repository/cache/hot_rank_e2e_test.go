//go:build e2e

package cache

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"testing"
	"time"
	"user-center/internal/config"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

func TestRedisHotRankCache_AtomicDedupAndStableSnapshot_e2e(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	c := newRedisHotRankCache(rdb, 3*time.Minute, time.Minute, time.Hour, "test:hot:"+uuid.NewString())
	snapshot := time.Now().Truncate(time.Minute)
	event1, event2, event3 := uuid.NewString(), uuid.NewString(), uuid.NewString()
	note1, note2 := time.Now().UnixNano(), time.Now().UnixNano()+1
	keys := []string{c.snapshotKey(snapshot), c.snapshotReadyKey(snapshot), c.eventKey(event1), c.eventKey(event2), c.eventKey(event3)}
	for i := 0; i < 3; i++ {
		keys = append(keys, c.bucketKey(snapshot.Add(-time.Duration(i)*time.Minute)))
	}
	defer rdb.Del(ctx, keys...)
	if err := rdb.Del(ctx, keys...).Err(); err != nil {
		t.Fatal(err)
	}

	applied, err := c.Record(ctx, event1, note1, snapshot, 10)
	if err != nil || !applied {
		t.Fatalf("first record applied=%v err=%v", applied, err)
	}
	applied, err = c.Record(ctx, event1, note1, snapshot, 10)
	if err != nil || applied {
		t.Fatalf("duplicate applied=%v err=%v", applied, err)
	}
	if _, err = c.Record(ctx, event2, note1, snapshot, 3); err != nil {
		t.Fatal(err)
	}
	if _, err = c.Record(ctx, event3, note2, snapshot, 5); err != nil {
		t.Fatal(err)
	}

	items, err := c.SnapshotPage(ctx, snapshot, 0, 10, true)
	if err != nil || len(items) != 2 || items[0].NoteID != note1 || items[0].Score != 13 || items[1].Score != 5 {
		t.Fatalf("snapshot items=%+v err=%v", items, err)
	}
	lateEvent := uuid.NewString()
	defer rdb.Del(ctx, c.eventKey(lateEvent))
	if _, err = c.Record(ctx, lateEvent, note2, snapshot, 100); err != nil {
		t.Fatal(err)
	}
	items, err = c.SnapshotPage(ctx, snapshot, 0, 10, false)
	if err != nil || len(items) != 2 || items[0].NoteID != note1 || items[1].Score != 5 {
		t.Fatalf("materialized snapshot changed: items=%+v err=%v", items, err)
	}
}

func BenchmarkRedisHotRankSnapshot_e2e(b *testing.B) {
	mgr, err := config.NewManager(filepath.Join("..", "..", "..", "config", "dev.yaml"))
	if err != nil {
		b.Fatal(err)
	}
	cfg := mgr.App()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	ctx := context.Background()
	if err = rdb.Ping(ctx).Err(); err != nil {
		b.Skipf("redis not available: %v", err)
	}
	const (
		bucketCount = 60
		noteCount   = 100
	)
	c := newRedisHotRankCache(rdb, bucketCount*time.Minute, time.Minute, time.Hour, "bench:hot:"+uuid.NewString())
	snapshot := time.Now().Truncate(time.Minute)
	keys := []string{c.snapshotKey(snapshot), c.snapshotReadyKey(snapshot)}
	pipe := rdb.Pipeline()
	for minute := 0; minute < bucketCount; minute++ {
		key := c.bucketKey(snapshot.Add(-time.Duration(minute) * time.Minute))
		keys = append(keys, key)
		members := make([]redis.Z, 0, noteCount)
		for noteID := 1; noteID <= noteCount; noteID++ {
			members = append(members, redis.Z{Member: strconv.Itoa(noteID), Score: float64((minute + 1) * (noteID%10 + 1))})
		}
		pipe.ZAdd(ctx, key, members...)
	}
	if _, err = pipe.Exec(ctx); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = rdb.Del(ctx, keys...).Err() })

	b.Run("cold_snapshot_top20", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			b.StopTimer()
			if err := rdb.Del(ctx, c.snapshotKey(snapshot), c.snapshotReadyKey(snapshot)).Err(); err != nil {
				b.Fatal(err)
			}
			b.StartTimer()
			if _, err := c.SnapshotPage(ctx, snapshot, 0, 20, true); err != nil {
				b.Fatal(err)
			}
		}
	})
	if _, err = c.SnapshotPage(ctx, snapshot, 0, 20, true); err != nil {
		b.Fatal(err)
	}
	b.Run("materialized_top20", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			if _, err := c.SnapshotPage(ctx, snapshot, 0, 20, false); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestRedisHotRankCache_Expiry_e2e(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	c := newRedisHotRankCache(rdb, time.Minute, 30*time.Millisecond, time.Hour, "test:hot:"+uuid.NewString())
	snapshot := time.Now().Truncate(time.Minute)
	eventID := uuid.NewString()
	keys := []string{c.snapshotKey(snapshot), c.snapshotReadyKey(snapshot), c.bucketKey(snapshot), c.eventKey(eventID)}
	defer rdb.Del(ctx, keys...)
	_ = rdb.Del(ctx, keys...).Err()
	if _, err := c.Record(ctx, eventID, 99, snapshot, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SnapshotPage(ctx, snapshot, 0, 10, true); err != nil {
		t.Fatal(err)
	}
	eventuallyRedis(t, time.Second, func() (bool, error) {
		exists, err := rdb.Exists(ctx, c.snapshotReadyKey(snapshot)).Result()
		return exists == 0, err
	})
	if _, err := c.SnapshotPage(ctx, snapshot, 0, 10, false); !errors.Is(err, ErrHotRankSnapshotExpired) {
		t.Fatalf("want ErrHotRankSnapshotExpired, got %v", err)
	}

	oldEvent := uuid.NewString()
	oldBucket := snapshot.Add(-2 * time.Minute)
	defer rdb.Del(ctx, c.eventKey(oldEvent), c.bucketKey(oldBucket))
	if _, err := c.Record(ctx, oldEvent, 100, oldBucket, 1); err != nil {
		t.Fatal(err)
	}
	eventuallyRedis(t, time.Second, func() (bool, error) {
		exists, err := rdb.Exists(ctx, c.bucketKey(oldBucket)).Result()
		return exists == 0, err
	})
}

func eventuallyRedis(t *testing.T, timeout time.Duration, condition func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		ok, err := condition()
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal(fmt.Errorf("condition not met within %s", timeout))
		}
		timer := time.NewTimer(10 * time.Millisecond)
		<-timer.C
	}
}
