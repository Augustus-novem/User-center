package repository

import (
	"context"
	"time"
	"user-center/internal/repository/cache"
)

type HotRankRepository interface {
	Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error)
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
