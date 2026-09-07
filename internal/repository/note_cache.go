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
	logger         logger.Logger
	group          singleflight.Group
	coalesceMisses bool
}

func NewCachedNoteRepository(inner *NoteRepositoryImpl, noteCache cache.NoteCache, l logger.Logger) *CachedNoteRepository {
	return newCachedNoteRepository(inner, noteCache, l, true)
}

func newCachedNoteRepository(inner *NoteRepositoryImpl, noteCache cache.NoteCache, l logger.Logger, coalesceMisses bool) *CachedNoteRepository {
	return &CachedNoteRepository{
		inner:          inner,
		cache:          noteCache,
		logger:         l,
		coalesceMisses: coalesceMisses,
	}
}

func (r *CachedNoteRepository) Create(ctx context.Context, note domain.Note) (domain.Note, error) {
	created, err := r.inner.Create(ctx, note)
	if err != nil {
		return domain.Note{}, err
	}
	r.invalidate(ctx, created.ID)
	return created, nil
}

func (r *CachedNoteRepository) FindByID(ctx context.Context, id int64) (domain.Note, error) {
	note, err := r.cache.Get(ctx, id)
	if err == nil {
		return note, nil
	}
	if errors.Is(err, cache.ErrNoteNotFound) {
		return domain.Note{}, ErrNoteNotFound
	}
	if !r.coalesceMisses {
		return r.loadAndCache(ctx, id)
	}
	result := r.group.DoChan(strconv.FormatInt(id, 10), func() (any, error) {
		// A previous leader may have populated Redis before this goroutine joined.
		note, cacheErr := r.cache.Get(ctx, id)
		if cacheErr == nil {
			return note, nil
		}
		if errors.Is(cacheErr, cache.ErrNoteNotFound) {
			return domain.Note{}, ErrNoteNotFound
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

func (r *CachedNoteRepository) loadAndCache(ctx context.Context, id int64) (domain.Note, error) {
	note, err := r.inner.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNoteNotFound) || errors.Is(err, ErrNoteDeleted) {
			if cacheErr := r.cache.SetNotFound(ctx, id); cacheErr != nil {
				r.logger.Warn("write negative note cache failed", logger.Error(cacheErr))
			}
		}
		return domain.Note{}, err
	}
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
	if err := r.inner.SoftDelete(ctx, id, authorID); err != nil {
		return err
	}
	r.invalidate(ctx, id)
	return nil
}

func (r *CachedNoteRepository) invalidate(ctx context.Context, id int64) {
	if err := r.cache.Delete(ctx, id); err != nil {
		r.logger.Warn("invalidate note cache failed", logger.Error(err))
	}
}
