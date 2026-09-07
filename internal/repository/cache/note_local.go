package cache

import (
	"sync"
	"time"
	"user-center/internal/domain"
)

const (
	noteLocalCacheTTL        = time.Second
	noteLocalCacheMaxEntries = 1024
)

type NoteLocalCache interface {
	Get(id int64) (domain.Note, error)
	Set(note domain.Note)
	SetNotFound(id int64)
	Delete(id int64)
}

type noteLocalEntry struct {
	note      domain.Note
	notFound  bool
	expiresAt time.Time
}

type MemoryNoteLocalCache struct {
	mu         sync.Mutex
	entries    map[int64]noteLocalEntry
	ttl        time.Duration
	maxEntries int
	now        func() time.Time
}

func NewMemoryNoteLocalCache() *MemoryNoteLocalCache {
	return newMemoryNoteLocalCache(noteLocalCacheTTL, noteLocalCacheMaxEntries, time.Now)
}

func newMemoryNoteLocalCache(ttl time.Duration, maxEntries int, now func() time.Time) *MemoryNoteLocalCache {
	return &MemoryNoteLocalCache{
		entries:    make(map[int64]noteLocalEntry, maxEntries),
		ttl:        ttl,
		maxEntries: maxEntries,
		now:        now,
	}
}

func (c *MemoryNoteLocalCache) Get(id int64) (domain.Note, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry, ok := c.entries[id]
	if !ok {
		return domain.Note{}, ErrNoteCacheMiss
	}
	if !c.now().Before(entry.expiresAt) {
		delete(c.entries, id)
		return domain.Note{}, ErrNoteCacheMiss
	}
	if entry.notFound {
		return domain.Note{}, ErrNoteNotFound
	}
	return cloneNote(entry.note), nil
}

func (c *MemoryNoteLocalCache) Set(note domain.Note) {
	c.store(note.ID, noteLocalEntry{
		note:      cloneNote(note),
		expiresAt: c.now().Add(c.ttl),
	})
}

func (c *MemoryNoteLocalCache) SetNotFound(id int64) {
	c.store(id, noteLocalEntry{
		notFound:  true,
		expiresAt: c.now().Add(c.ttl),
	})
}

func (c *MemoryNoteLocalCache) Delete(id int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, id)
}

func (c *MemoryNoteLocalCache) store(id int64, entry noteLocalEntry) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[id]; !exists && len(c.entries) >= c.maxEntries {
		c.evictEarliestExpiry()
	}
	c.entries[id] = entry
}

func (c *MemoryNoteLocalCache) evictEarliestExpiry() {
	var candidateID int64
	var candidateExpiry time.Time
	first := true
	for id, entry := range c.entries {
		if first || entry.expiresAt.Before(candidateExpiry) {
			candidateID = id
			candidateExpiry = entry.expiresAt
			first = false
		}
	}
	if !first {
		delete(c.entries, candidateID)
	}
}

func cloneNote(note domain.Note) domain.Note {
	note.Images = append([]domain.NoteImage(nil), note.Images...)
	return note
}
