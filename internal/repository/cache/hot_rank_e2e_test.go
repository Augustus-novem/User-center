//go:build e2e

package cache

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRedisHotRankCache_AtomicDedupAndStableSnapshot_e2e(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	c := NewRedisHotRankCache(rdb, 3*time.Minute, time.Minute, time.Hour)
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

func TestRedisHotRankCache_Expiry_e2e(t *testing.T) {
	rdb := newTestRedis(t)
	ctx := context.Background()
	c := NewRedisHotRankCache(rdb, time.Minute, 30*time.Millisecond, time.Hour)
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
