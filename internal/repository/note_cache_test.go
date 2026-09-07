package repository

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/pkg/logger"
)

type noteCacheStub struct {
	getFn         func(ctx context.Context, id int64) (domain.Note, error)
	setFn         func(ctx context.Context, note domain.Note) error
	setNotFoundFn func(ctx context.Context, id int64) error
	deleteFn      func(ctx context.Context, id int64) error
}

func newCachedNoteRepositoryForTest(inner *NoteRepositoryImpl, noteCache cache.NoteCache, l logger.Logger) *CachedNoteRepository {
	return NewCachedNoteRepository(inner, noteCache, cache.NewMemoryNoteLocalCache(), l)
}

func (s *noteCacheStub) Get(ctx context.Context, id int64) (domain.Note, error) {
	return s.getFn(ctx, id)
}

func (s *noteCacheStub) Set(ctx context.Context, note domain.Note) error {
	if s.setFn == nil {
		return nil
	}
	return s.setFn(ctx, note)
}

func (s *noteCacheStub) SetNotFound(ctx context.Context, id int64) error {
	if s.setNotFoundFn == nil {
		return nil
	}
	return s.setNotFoundFn(ctx, id)
}

func (s *noteCacheStub) Delete(ctx context.Context, id int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, id)
}

func TestCachedNoteRepository_FindByIDHit(t *testing.T) {
	t.Parallel()
	want := domain.Note{ID: 9, Title: "cache hit"}
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			t.Fatal("database must not be called on cache hit")
			return dao.NoteOfDB{}, nil
		},
	}), &noteCacheStub{getFn: func(ctx context.Context, id int64) (domain.Note, error) {
		return want, nil
	}}, logger.NewNoOpLogger())

	got, err := repo.FindByID(context.Background(), want.ID)
	if err != nil || got.ID != want.ID || got.Title != want.Title {
		t.Fatalf("note=%+v err=%v", got, err)
	}
}

func TestCachedNoteRepository_FindByIDMissLoadsAndCaches(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("cache miss")
	var cached domain.Note
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			return dao.NoteOfDB{Id: id, AuthorId: 2, Title: "database", Status: domain.NoteStatusPublished}, nil
		},
	}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, wantErr
		},
		setFn: func(ctx context.Context, note domain.Note) error {
			cached = note
			return nil
		},
	}, logger.NewNoOpLogger())

	got, err := repo.FindByID(context.Background(), 4)
	if err != nil || got.ID != 4 || cached.ID != 4 {
		t.Fatalf("note=%+v cached=%+v err=%v", got, cached, err)
	}
}

func TestCachedNoteRepository_FindByIDNegativeHitSkipsDatabase(t *testing.T) {
	t.Parallel()
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			t.Fatal("database must not be called on negative cache hit")
			return dao.NoteOfDB{}, nil
		},
	}), &noteCacheStub{getFn: func(ctx context.Context, id int64) (domain.Note, error) {
		return domain.Note{}, cache.ErrNoteNotFound
	}}, logger.NewNoOpLogger())

	_, err := repo.FindByID(context.Background(), 404)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("want ErrNoteNotFound, got %v", err)
	}
}

func TestCachedNoteRepository_FindByIDNotFoundWritesNegativeEntry(t *testing.T) {
	t.Parallel()
	var negativeID int64
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, errors.New("cache miss")
		},
		setNotFoundFn: func(ctx context.Context, id int64) error {
			negativeID = id
			return nil
		},
	}, logger.NewNoOpLogger())

	_, err := repo.FindByID(context.Background(), 404)
	if !errors.Is(err, ErrNoteNotFound) || negativeID != 404 {
		t.Fatalf("err=%v negativeID=%d", err, negativeID)
	}
}

func TestCachedNoteRepository_CoalescesConcurrentMisses(t *testing.T) {
	const callers = 64
	var originCalls atomic.Int64
	var mu sync.RWMutex
	var stored *domain.Note
	cacheMiss := errors.New("cache miss")
	noteCache := &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			mu.RLock()
			defer mu.RUnlock()
			if stored == nil {
				return domain.Note{}, cacheMiss
			}
			return *stored, nil
		},
		setFn: func(ctx context.Context, note domain.Note) error {
			mu.Lock()
			defer mu.Unlock()
			copyOfNote := note
			stored = &copyOfNote
			return nil
		},
	}
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			originCalls.Add(1)
			time.Sleep(20 * time.Millisecond)
			return dao.NoteOfDB{Id: id, Status: domain.NoteStatusPublished}, nil
		},
	}), noteCache, logger.NewNoOpLogger())

	start := make(chan struct{})
	errCh := make(chan error, callers)
	var wg sync.WaitGroup
	wg.Add(callers)
	for range callers {
		go func() {
			defer wg.Done()
			<-start
			note, err := repo.FindByID(context.Background(), 21)
			if err != nil {
				errCh <- err
				return
			}
			if note.ID != 21 {
				errCh <- errors.New("unexpected note ID")
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	if got := originCalls.Load(); got != 1 {
		t.Fatalf("want one origin call, got %d", got)
	}
}

func TestCachedNoteRepository_LocalHitSkipsRedis(t *testing.T) {
	t.Parallel()
	var redisReads atomic.Int64
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			redisReads.Add(1)
			return domain.Note{ID: id, Title: "redis"}, nil
		},
	}, logger.NewNoOpLogger())

	for range 2 {
		note, err := repo.FindByID(context.Background(), 8)
		if err != nil || note.Title != "redis" {
			t.Fatalf("note=%+v err=%v", note, err)
		}
	}
	if got := redisReads.Load(); got != 1 {
		t.Fatalf("want one Redis read, got %d", got)
	}
}

func TestCachedNoteRepository_RedisFailureFallsBackToOrigin(t *testing.T) {
	t.Parallel()
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			return dao.NoteOfDB{Id: id, Title: "origin", Status: domain.NoteStatusPublished}, nil
		},
	}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, errors.New("redis unavailable")
		},
		setFn: func(ctx context.Context, note domain.Note) error {
			return errors.New("redis unavailable")
		},
	}, logger.NewNoOpLogger())

	note, err := repo.FindByID(context.Background(), 18)
	if err != nil || note.ID != 18 || note.Title != "origin" {
		t.Fatalf("note=%+v err=%v", note, err)
	}
}

func TestCachedNoteRepository_DeleteInvalidatesLocalCache(t *testing.T) {
	t.Parallel()
	local := cache.NewMemoryNoteLocalCache()
	local.Set(domain.Note{ID: 31})
	repo := NewCachedNoteRepository(NewNoteRepositoryImpl(&noteDAOStub{}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, cache.ErrNoteCacheMiss
		},
	}, local, logger.NewNoOpLogger())

	if err := repo.SoftDelete(context.Background(), 31, 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := local.Get(31); !errors.Is(err, cache.ErrNoteCacheMiss) {
		t.Fatalf("want local miss after delete, got %v", err)
	}
}

func TestCachedNoteRepository_CreateAndDeleteInvalidate(t *testing.T) {
	t.Parallel()
	var invalidated []int64
	repo := newCachedNoteRepositoryForTest(NewNoteRepositoryImpl(&noteDAOStub{
		insertFn: func(ctx context.Context, note dao.NoteOfDB) (dao.NoteOfDB, error) {
			note.Id = 6
			return note, nil
		},
	}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, errors.New("unused")
		},
		deleteFn: func(ctx context.Context, id int64) error {
			invalidated = append(invalidated, id)
			return nil
		},
	}, logger.NewNoOpLogger())

	created, err := repo.Create(context.Background(), domain.Note{AuthorID: 2})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err = repo.SoftDelete(context.Background(), created.ID, 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(invalidated) != 2 || invalidated[0] != 6 || invalidated[1] != 6 {
		t.Fatalf("invalidated=%v", invalidated)
	}
}
