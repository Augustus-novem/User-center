package repository

import (
	"context"
	"errors"
	"strconv"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/pkg/logger"

	"golang.org/x/sync/singleflight"
)

type CachedNoteRepository struct {
	inner          *NoteRepositoryImpl
	cache          cache.NoteCache
	local          cache.NoteLocalCache
	logger         logger.Logger
	group          singleflight.Group
	coalesceMisses bool
}

func NewCachedNoteRepository(inner *NoteRepositoryImpl, noteCache cache.NoteCache, local cache.NoteLocalCache, l logger.Logger) *CachedNoteRepository {
	return newCachedNoteRepository(inner, noteCache, local, l, true)
}

func newCachedNoteRepository(inner *NoteRepositoryImpl, noteCache cache.NoteCache, local cache.NoteLocalCache, l logger.Logger, coalesceMisses bool) *CachedNoteRepository {
	return &CachedNoteRepository{
		inner:          inner,
		cache:          noteCache,
		local:          local,
		logger:         l,
		coalesceMisses: coalesceMisses,
	}
}

func (r *CachedNoteRepository) Create(ctx context.Context, note domain.Note) (domain.Note, error) {
	created, err := r.inner.Create(ctx, note)
	if err != nil {
		return domain.Note{}, err
	}
	return created, nil
}

func (r *CachedNoteRepository) FindByID(ctx context.Context, id int64) (domain.Note, error) {
	if note, err, handled := r.lookupCaches(ctx, id); handled {
		return note, err
	}
	if !r.coalesceMisses {
		return r.loadAndCache(ctx, id)
	}
	result := r.group.DoChan(strconv.FormatInt(id, 10), func() (any, error) {
		// A previous leader may have populated Redis before this goroutine joined.
		if note, err, handled := r.lookupCaches(ctx, id); handled {
			return note, err
		}
		return r.loadAndCache(ctx, id)
	})
	select {
	case call := <-result:
		if call.Err != nil {
			return domain.Note{}, call.Err
		}
		return call.Val.(domain.Note), nil
	case <-ctx.Done():
		return domain.Note{}, ctx.Err()
	}
}

func (r *CachedNoteRepository) lookupCaches(ctx context.Context, id int64) (domain.Note, error, bool) {
	note, err := r.local.Get(id)
	if err == nil {
		return note, nil, true
	}
	if errors.Is(err, cache.ErrNoteNotFound) {
		return domain.Note{}, ErrNoteNotFound, true
	}

	note, err = r.cache.Get(ctx, id)
	if err == nil {
		r.local.Set(note)
		return note, nil, true
	}
	if errors.Is(err, cache.ErrNoteNotFound) {
		r.local.SetNotFound(id)
		return domain.Note{}, ErrNoteNotFound, true
	}
	if !errors.Is(err, cache.ErrNoteCacheMiss) {
		r.logger.Warn("read note cache failed", logger.Error(err))
	}
	return domain.Note{}, nil, false
}

func (r *CachedNoteRepository) loadAndCache(ctx context.Context, id int64) (domain.Note, error) {
	note, err := r.inner.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) || errors.Is(err, ErrNoteDeleted) {
			r.local.SetNotFound(id)
			if cacheErr := r.cache.SetNotFound(ctx, id); cacheErr != nil {
				r.logger.Warn("write negative note cache failed", logger.Error(cacheErr))
			}
		}
		return domain.Note{}, err
	}
	r.local.Set(note)
	if cacheErr := r.cache.Set(ctx, note); cacheErr != nil {
		r.logger.Warn("write note cache failed", logger.Error(cacheErr))
	}
	return note, nil
}

func (r *CachedNoteRepository) FindByIDs(ctx context.Context, ids []int64) (map[int64]domain.Note, error) {
	return r.inner.FindByIDs(ctx, ids)
}

func (r *CachedNoteRepository) ListByAuthor(ctx context.Context, authorID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
	return r.inner.ListByAuthor(ctx, authorID, cursor, limit)
}

func (r *CachedNoteRepository) ListPublishedBefore(ctx context.Context, authorID, exclusiveMaxID int64, limit int) ([]domain.Note, error) {
	return r.inner.ListPublishedBefore(ctx, authorID, exclusiveMaxID, limit)
}

func (r *CachedNoteRepository) SoftDelete(ctx context.Context, id, authorID int64) error {
	return r.inner.SoftDelete(ctx, id, authorID)
}

func (r *CachedNoteRepository) Invalidate(ctx context.Context, id int64) {
	r.local.Delete(id)
	if err := r.cache.Delete(ctx, id); err != nil {
		r.logger.Warn("invalidate note cache failed", logger.Error(err))
	}
}
