package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"user-center/internal/repository/cache"
)

func TestRankRepositoryImpl_TopNDaily(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 4, 10, 0, 0, 0, 0, time.Local)
	repo := NewRankRepositoryImpl(&rankCacheStub{
		topNDailyFn: func(ctx context.Context, gotDay time.Time, limit int64) ([]cache.RankUserScore, error) {
			if !gotDay.Equal(day) {
				t.Fatalf("unexpected day: %v", gotDay)
			}
			if limit != 5 {
				t.Fatalf("unexpected limit: %d", limit)
			}
			return []cache.RankUserScore{{UserID: 1, Score: 13}, {UserID: 2, Score: 8}}, nil
		},
	})

	items, err := repo.TopNDaily(context.Background(), day, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 || items[0].UserID != 1 || items[0].Score != 13 || items[1].UserID != 2 || items[1].Score != 8 {
		t.Fatalf("unexpected items: %+v", items)
	}
}

func TestRankRepositoryImpl_GetMonthlyRank(t *testing.T) {
	t.Parallel()

	repo := NewRankRepositoryImpl(&rankCacheStub{
		getMonthlyRankFn: func(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
			if userID != 88 || year != 2026 || month != time.April {
				t.Fatalf("unexpected args: uid=%d year=%d month=%d", userID, year, month)
			}
			return 4, 21, true, nil
		},
	})

	rank, score, found, err := repo.GetMonthlyRank(context.Background(), 88, 2026, time.April)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !found || rank != 4 || score != 21 {
		t.Fatalf("unexpected result: rank=%d score=%v found=%v", rank, score, found)
	}
}

func TestRankRepositoryImpl_IncrSignInScore(t *testing.T) {
	t.Parallel()

	when := time.Date(2026, 4, 10, 9, 30, 0, 0, time.Local)
	called := false
	repo := NewRankRepositoryImpl(&rankCacheStub{
		incrSignInScoreFn: func(ctx context.Context, userID int64, gotWhen time.Time, delta int64) error {
			called = true
			if userID != 7 || !gotWhen.Equal(when) || delta != 5 {
				t.Fatalf("unexpected args: uid=%d when=%v delta=%d", userID, gotWhen, delta)
			}
			return nil
		},
	})

	if err := repo.IncrSignInScore(context.Background(), 7, when, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("cache.IncrSignInScore should be called")
	}
}

func TestRankRepositoryImpl_ErrorPassThrough(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("redis down")
	repo := NewRankRepositoryImpl(&rankCacheStub{
		topNMonthlyFn: func(ctx context.Context, year int, month time.Month, limit int64) ([]cache.RankUserScore, error) {
			return nil, wantErr
		},
	})

	_, err := repo.TopNMonthly(context.Background(), 2026, time.April, 10)
	if !errors.Is(err, wantErr) {
		t.Fatalf("want err %v, got %v", wantErr, err)
	}
}

type rankCacheStub struct {
	incrSignInScoreFn func(ctx context.Context, userID int64, when time.Time, delta int64) error
	topNDailyFn       func(ctx context.Context, day time.Time, limit int64) ([]cache.RankUserScore, error)
	topNMonthlyFn     func(ctx context.Context, year int, month time.Month, limit int64) ([]cache.RankUserScore, error)
	getDailyRankFn    func(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error)
	getMonthlyRankFn  func(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error)
}

func (s *rankCacheStub) IncrSignInScore(ctx context.Context, userID int64, when time.Time, delta int64) error {
	if s.incrSignInScoreFn == nil {
		return nil
	}
	return s.incrSignInScoreFn(ctx, userID, when, delta)
}
func (s *rankCacheStub) TopNDaily(ctx context.Context, day time.Time, limit int64) ([]cache.RankUserScore, error) {
	if s.topNDailyFn == nil {
		return nil, nil
	}
	return s.topNDailyFn(ctx, day, limit)
}
func (s *rankCacheStub) TopNMonthly(ctx context.Context, year int, month time.Month, limit int64) ([]cache.RankUserScore, error) {
	if s.topNMonthlyFn == nil {
		return nil, nil
	}
	return s.topNMonthlyFn(ctx, year, month, limit)
}
func (s *rankCacheStub) GetDailyRank(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error) {
	if s.getDailyRankFn == nil {
		return 0, 0, false, nil
	}
	return s.getDailyRankFn(ctx, userID, day)
}
func (s *rankCacheStub) GetMonthlyRank(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
	if s.getMonthlyRankFn == nil {
		return 0, 0, false, nil
	}
	return s.getMonthlyRankFn(ctx, userID, year, month)
}
