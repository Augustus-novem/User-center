package service

import (
	"context"
	"testing"
	"time"

	"user-center/internal/domain"
	"user-center/internal/repository"
)

func TestRankServiceImpl_GetDailyTopN(t *testing.T) {
	t.Parallel()

	svc := NewRankServiceImpl(&rankRepoStub{
		topNDailyFn: func(ctx context.Context, day time.Time, limit int64) ([]repository.RankItem, error) {
			if limit != 3 {
				t.Fatalf("unexpected limit: %d", limit)
			}
			return []repository.RankItem{
				{UserID: 10, Score: 50000000012345},
				{UserID: 11, Score: 50000000001234},
			}, nil
		},
	}, &userRepoStub{
		findByIDFn: func(ctx context.Context, id int64) (domain.User, error) {
			return domain.User{Id: id}, nil
		},
	})

	res, err := svc.GetDailyTopN(context.Background(), 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res) != 2 {
		t.Fatalf("want 2 items, got %d", len(res))
	}
	if res[0].Rank != 1 || res[1].Rank != 2 {
		t.Fatalf("unexpected ranks: %+v", res)
	}
	if res[0].Score != signInPoints || res[1].Score != signInPoints {
		t.Fatalf("daily rank should expose fixed sign-in points instead of raw redis score, got %+v", res)
	}
	if res[0].DisplayName != "用户：10" || res[1].DisplayName != "用户：11" {
		t.Fatalf("unexpected display names: %+v", res)
	}
}

func TestRankServiceImpl_GetMonthlyMe(t *testing.T) {
	t.Parallel()

	t.Run("found user returns stored score and rank", func(t *testing.T) {
		t.Parallel()
		svc := NewRankServiceImpl(&rankRepoStub{
			getMonthlyRankFn: func(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
				return 4, 25, true, nil
			},
		}, &userRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.User, error) {
				return domain.User{Id: id}, nil
			},
		})
		res, err := svc.GetMonthlyMe(context.Background(), 99)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.UserID != 99 || res.Rank != 4 || res.Score != 25 {
			t.Fatalf("unexpected result: %+v", res)
		}
	})

	t.Run("not found returns zero rank and zero score", func(t *testing.T) {
		t.Parallel()
		svc := NewRankServiceImpl(&rankRepoStub{
			getMonthlyRankFn: func(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
				return 0, 0, false, nil
			},
		}, &userRepoStub{})
		res, err := svc.GetMonthlyMe(context.Background(), 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.UserID != 100 || res.Rank != 0 || res.Score != 0 {
			t.Fatalf("unexpected result: %+v", res)
		}
	})
}
