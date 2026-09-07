package repository

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/pkg/logger"
)

const noteCacheComparisonCallers = 64

type comparisonNoteCache struct {
	firstReads atomic.Int64
	ready      chan struct{}
	once       sync.Once
}

func newComparisonNoteCache() *comparisonNoteCache {
	return &comparisonNoteCache{ready: make(chan struct{})}
}

func (c *comparisonNoteCache) Get(context.Context, int64) (domain.Note, error) {
	read := c.firstReads.Add(1)
	if read <= noteCacheComparisonCallers {
		if read == noteCacheComparisonCallers {
			c.once.Do(func() { close(c.ready) })
		}
		<-c.ready
	}
	return domain.Note{}, cache.ErrNoteCacheMiss
}

func (*comparisonNoteCache) Set(context.Context, domain.Note) error   { return nil }
func (*comparisonNoteCache) SetNotFound(context.Context, int64) error { return nil }
func (*comparisonNoteCache) Delete(context.Context, int64) error      { return nil }

type comparisonResult struct {
	originLoads int64
	dbQueries   int64
	elapsed     time.Duration
}

func TestNoteCacheSingleflightComparison(t *testing.T) {
	results := map[string]comparisonResult{
		"OFF": runNoteCacheComparison(t, false),
		"ON":  runNoteCacheComparison(t, true),
	}
	for _, mode := range []string{"OFF", "ON"} {
		result := results[mode]
		t.Logf("singleflight=%s callers=%d origin_loads=%d db_queries=%d elapsed=%s",
			mode, noteCacheComparisonCallers, result.originLoads, result.dbQueries, result.elapsed)
	}
	if got := results["OFF"].dbQueries; got != noteCacheComparisonCallers*2 {
		t.Fatalf("singleflight OFF: want %d DB queries, got %d", noteCacheComparisonCallers*2, got)
	}
	if got := results["ON"].dbQueries; got != 2 {
		t.Fatalf("singleflight ON: want 2 DB queries, got %d", got)
	}
}

func runNoteCacheComparison(t *testing.T, singleflightEnabled bool) comparisonResult {
	t.Helper()
	var originLoads atomic.Int64
	var dbQueries atomic.Int64
	dbSlots := make(chan struct{}, 8)
	runQuery := func() {
		dbSlots <- struct{}{}
		dbQueries.Add(1)
		time.Sleep(5 * time.Millisecond)
		<-dbSlots
	}
	inner := NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			originLoads.Add(1)
			runQuery()
			return dao.NoteOfDB{Id: id, Title: "origin", Status: domain.NoteStatusPublished}, nil
		},
		listImagesFn: func(ctx context.Context, noteID int64) ([]dao.NoteImageOfDB, error) {
			runQuery()
			return nil, nil
		},
	})
	repo := newCachedNoteRepository(
		inner,
		newComparisonNoteCache(),
		cache.NewMemoryNoteLocalCache(),
		logger.NewNoOpLogger(),
		singleflightEnabled,
	)

	start := make(chan struct{})
	errCh := make(chan error, noteCacheComparisonCallers)
	var wg sync.WaitGroup
	wg.Add(noteCacheComparisonCallers)
	startedAt := time.Now()
	for range noteCacheComparisonCallers {
		go func() {
			defer wg.Done()
			<-start
			note, err := repo.FindByID(context.Background(), 77)
			if err != nil {
				errCh <- err
				return
			}
			if note.ID != 77 {
				errCh <- fmt.Errorf("unexpected note ID %d", note.ID)
			}
		}()
	}
	close(start)
	wg.Wait()
	elapsed := time.Since(startedAt)
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	return comparisonResult{
		originLoads: originLoads.Load(),
		dbQueries:   dbQueries.Load(),
		elapsed:     elapsed,
	}
}
