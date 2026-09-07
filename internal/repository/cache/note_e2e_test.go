//go:build e2e

package cache

import (
	"context"
	"errors"
	"testing"
	"time"
	"user-center/internal/domain"
)

func TestRedisNoteCache_RoundTrip_e2e(t *testing.T) {
	rdb := newTestRedis(t)
	c := NewRedisNoteCache(rdb)
	ctx := context.Background()
	id := time.Now().UnixNano()
	defer rdb.Del(ctx, c.key(id))

	want := domain.Note{
		ID:       id,
		AuthorID: 9,
		Title:    "redis integration",
		Images:   []domain.NoteImage{{URL: "https://example.com/image.png"}},
	}
	if err := c.Set(ctx, want); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := c.Get(ctx, id)
	if err != nil || got.ID != want.ID || got.Title != want.Title || len(got.Images) != 1 {
		t.Fatalf("note=%+v err=%v", got, err)
	}
	ttl, err := rdb.TTL(ctx, c.key(id)).Result()
	if err != nil || !isWithinJitter(ttl, noteCacheTTL) {
		t.Fatalf("positive ttl=%v err=%v", ttl, err)
	}

	if err = c.SetNotFound(ctx, id); err != nil {
		t.Fatalf("set not found: %v", err)
	}
	if _, err = c.Get(ctx, id); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("want negative entry, got %v", err)
	}
	if err = c.Delete(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err = c.Get(ctx, id); !errors.Is(err, ErrNoteCacheMiss) {
		t.Fatalf("want miss after delete, got %v", err)
	}
}
