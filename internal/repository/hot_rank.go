package repository

import (
	"context"
	"time"
	"user-center/internal/repository/cache"
)

var ErrHotRankSnapshotExpired = cache.ErrHotRankSnapshotExpired

type HotRankRepository interface {
	Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error)
	SnapshotPage(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]HotRankItem, error)
}

type HotRankItem struct {
	NoteID int64
	Score  float64
}

type HotRankRepositoryImpl struct {
	cache cache.HotRankCache
}

func NewHotRankRepositoryImpl(hotCache cache.HotRankCache) *HotRankRepositoryImpl {
	return &HotRankRepositoryImpl{cache: hotCache}
}

func (r *HotRankRepositoryImpl) Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error) {
	return r.cache.Record(ctx, eventID, noteID, occurredAt, weight)
}

func (r *HotRankRepositoryImpl) SnapshotPage(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]HotRankItem, error) {
	items, err := r.cache.SnapshotPage(ctx, snapshot, offset, limit, create)
	if err != nil {
		return nil, err
	}
	result := make([]HotRankItem, 0, len(items))
	for _, item := range items {
		result = append(result, HotRankItem{NoteID: item.NoteID, Score: item.Score})
	}
	return result, nil
}
