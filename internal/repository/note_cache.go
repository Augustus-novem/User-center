package repository

import (
	"context"
	"errors"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/pkg/logger"
)

type CachedNoteRepository struct {
	inner  *NoteRepositoryImpl
	cache  cache.NoteCache
	logger logger.Logger
}

func NewCachedNoteRepository(inner *NoteRepositoryImpl, noteCache cache.NoteCache, l logger.Logger) *CachedNoteRepository {
	return &CachedNoteRepository{
		inner:  inner,
		cache:  noteCache,
		logger: l,
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
	note, err = r.inner.FindByID(ctx, id)
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
