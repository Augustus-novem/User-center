package repository

import (
	"context"
	"time"
	"user-center/internal/repository/cache"
)

type RankItem struct {
	UserID int64
	Score  float64
}

type RankRepository interface {
	IncrSignInScore(ctx context.Context, userID int64, when time.Time, delta int64) error
	TopNDaily(ctx context.Context, day time.Time, limit int64) ([]RankItem, error)
	TopNMonthly(ctx context.Context, year int, month time.Month, limit int64) ([]RankItem, error)
	GetDailyRank(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error)
	GetMonthlyRank(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error)
}

type RankRepositoryImpl struct {
	cache cache.RankCache
}

func NewRankRepositoryImpl(cache cache.RankCache) *RankRepositoryImpl {
	return &RankRepositoryImpl{cache: cache}
}

func (r *RankRepositoryImpl) TopNDaily(ctx context.Context, day time.Time, limit int64) ([]RankItem, error) {
	items, err := r.cache.TopNDaily(ctx, day, limit)
	if err != nil {
		return nil, err
	}
	res := make([]RankItem, 0, len(items))
	for _, item := range items {
		res = append(res, RankItem{UserID: item.UserID, Score: item.Score})
	}
	return res, nil
}

func (r *RankRepositoryImpl) TopNMonthly(ctx context.Context, year int, month time.Month, limit int64) ([]RankItem, error) {
	items, err := r.cache.TopNMonthly(ctx, year, month, limit)
	if err != nil {
		return nil, err
	}
	res := make([]RankItem, 0, len(items))
	for _, item := range items {
		res = append(res, RankItem{UserID: item.UserID, Score: item.Score})
	}
	return res, nil
}

func (r *RankRepositoryImpl) GetDailyRank(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error) {
	return r.cache.GetDailyRank(ctx, userID, day)
}

func (r *RankRepositoryImpl) GetMonthlyRank(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
	return r.cache.GetMonthlyRank(ctx, userID, year, month)
}

func (r *RankRepositoryImpl) IncrSignInScore(ctx context.Context, userID int64, when time.Time, delta int64) error {
	return r.cache.IncrSignInScore(ctx, userID, when, delta)
}
