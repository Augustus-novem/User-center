package cache

import (
	"errors"
	"testing"
	"time"
	"user-center/internal/domain"
)

func TestMemoryNoteLocalCache_HitExpiryAndDelete(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	c := newMemoryNoteLocalCache(time.Second, 2, func() time.Time { return now })
	note := domain.Note{ID: 1, Images: []domain.NoteImage{{URL: "original"}}}
	c.Set(note)

	got, err := c.Get(1)
	if err != nil || got.Images[0].URL != "original" {
		t.Fatalf("note=%+v err=%v", got, err)
	}
	got.Images[0].URL = "mutated"
	again, err := c.Get(1)
	if err != nil || again.Images[0].URL != "original" {
		t.Fatalf("cached value was mutated: note=%+v err=%v", again, err)
	}

	now = now.Add(time.Second)
	if _, err = c.Get(1); !errors.Is(err, ErrNoteCacheMiss) {
		t.Fatalf("want expired miss, got %v", err)
	}
	c.Set(domain.Note{ID: 1})
	c.Delete(1)
	if _, err = c.Get(1); !errors.Is(err, ErrNoteCacheMiss) {
		t.Fatalf("want deleted miss, got %v", err)
	}
}

func TestMemoryNoteLocalCache_NegativeAndCapacity(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 8, 0, 0, 0, 0, time.UTC)
	c := newMemoryNoteLocalCache(time.Second, 2, func() time.Time { return now })
	c.SetNotFound(1)
	if _, err := c.Get(1); !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("want negative hit, got %v", err)
	}

	now = now.Add(100 * time.Millisecond)
	c.Set(domain.Note{ID: 2})
	now = now.Add(100 * time.Millisecond)
	c.Set(domain.Note{ID: 3})
	if _, err := c.Get(1); !errors.Is(err, ErrNoteCacheMiss) {
		t.Fatalf("oldest entry should be evicted, got %v", err)
	}
	if _, err := c.Get(2); err != nil {
		t.Fatalf("entry 2 should remain: %v", err)
	}
	if _, err := c.Get(3); err != nil {
		t.Fatalf("entry 3 should remain: %v", err)
	}
}
