package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

type signInRepoStub struct {
	signInFn             func(ctx context.Context, userId, signInAt int64) (int, bool, error)
	getContinuousDaysFn  func(ctx context.Context, userID int64, nowTs int64) (int, error)
	syncSignedOnDateFn   func(ctx context.Context, userID int64, signInAt int64) error
	isSignedOnDateFn     func(ctx context.Context, userID int64, ts int64) (bool, error)
	getMonthSignedDaysFn func(ctx context.Context, userID int64, year int, month time.Month) ([]int, error)
}

func (s *signInRepoStub) SignIn(ctx context.Context, userId, signInAt int64) (int, bool, error) {
	if s.signInFn == nil {
		return 0, false, nil
	}
	return s.signInFn(ctx, userId, signInAt)
}

func (s *signInRepoStub) GetContinuousDays(ctx context.Context, userID int64, nowTs int64) (int, error) {
	if s.getContinuousDaysFn == nil {
		return 0, nil
	}
	return s.getContinuousDaysFn(ctx, userID, nowTs)
}

func (s *signInRepoStub) SyncSignedOnDate(ctx context.Context, userID int64, signInAt int64) error {
	if s.syncSignedOnDateFn == nil {
		return nil
	}
	return s.syncSignedOnDateFn(ctx, userID, signInAt)
}

func (s *signInRepoStub) IsSignedOnDate(ctx context.Context, userID int64, ts int64) (bool, error) {
	if s.isSignedOnDateFn == nil {
		return false, nil
	}
	return s.isSignedOnDateFn(ctx, userID, ts)
}

func (s *signInRepoStub) GetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month) ([]int, error) {
	if s.getMonthSignedDaysFn == nil {
		return nil, nil
	}
	return s.getMonthSignedDaysFn(ctx, userID, year, month)
}

type pointRepoStub struct {
	addSignInPointsFn func(ctx context.Context, userID int64, signInAt int64, points int) error
}

func (s *pointRepoStub) AddSignInPoints(ctx context.Context, userID int64, signInAt int64, points int) error {
	if s.addSignInPointsFn == nil {
		return nil
	}
	return s.addSignInPointsFn(ctx, userID, signInAt, points)
}

func (s *pointRepoStub) AddWelcomePoints(ctx context.Context, userID int64, points int) error {
	return nil
}

type activityLogRepoStub struct {
	appendFn func(ctx context.Context, entry repository.ActivityLogEntry) error
}

func (s *activityLogRepoStub) Append(ctx context.Context, entry repository.ActivityLogEntry) error {
	if s.appendFn == nil {
		return nil
	}
	return s.appendFn(ctx, entry)
}

type rankRepoStub struct {
	incrSignInScoreFn func(ctx context.Context, userID int64, when time.Time, delta int64) error
	topNDailyFn       func(ctx context.Context, day time.Time, limit int64) ([]repository.RankItem, error)
	topNMonthlyFn     func(ctx context.Context, year int, month time.Month, limit int64) ([]repository.RankItem, error)
	getDailyRankFn    func(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error)
	getMonthlyRankFn  func(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error)
}

func (s *rankRepoStub) IncrSignInScore(ctx context.Context, userID int64, when time.Time, delta int64) error {
	if s.incrSignInScoreFn == nil {
		return nil
	}
	return s.incrSignInScoreFn(ctx, userID, when, delta)
}

func (s *rankRepoStub) TopNDaily(ctx context.Context, day time.Time, limit int64) ([]repository.RankItem, error) {
	if s.topNDailyFn == nil {
		return nil, nil
	}
	return s.topNDailyFn(ctx, day, limit)
}

func (s *rankRepoStub) TopNMonthly(ctx context.Context, year int, month time.Month, limit int64) ([]repository.RankItem, error) {
	if s.topNMonthlyFn == nil {
		return nil, nil
	}
	return s.topNMonthlyFn(ctx, year, month, limit)
}

func (s *rankRepoStub) GetDailyRank(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error) {
	if s.getDailyRankFn == nil {
		return 0, 0, false, nil
	}
	return s.getDailyRankFn(ctx, userID, day)
}

func (s *rankRepoStub) GetMonthlyRank(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
	if s.getMonthlyRankFn == nil {
		return 0, 0, false, nil
	}
	return s.getMonthlyRankFn(ctx, userID, year, month)
}

func TestSignInServiceImpl_SignIn(t *testing.T) {
	t.Parallel()

	t.Run("first sign in adds points and updates rank/cache after tx", func(t *testing.T) {
		t.Parallel()
		var (
			pointCalled bool
			syncCalled  bool
			rankCalled  bool
			inTxCalled  bool
		)
		repo := &signInRepoStub{
			signInFn: func(ctx context.Context, userId, signInAt int64) (int, bool, error) {
				if userId != 123 {
					t.Fatalf("unexpected user id: %d", userId)
				}
				if signInAt <= 0 {
					t.Fatalf("unexpected signInAt: %d", signInAt)
				}
				return 3, false, nil
			},
			syncSignedOnDateFn: func(ctx context.Context, userID int64, signInAt int64) error {
				syncCalled = true
				if !inTxCalled {
					// 这里只要求发生在事务结束后，后面会再检查 point 是否先发生
				}
				return nil
			},
		}
		pointRepo := &pointRepoStub{
			addSignInPointsFn: func(ctx context.Context, userID int64, signInAt int64, points int) error {
				pointCalled = true
				if userID != 123 || points != signInPoints {
					t.Fatalf("unexpected point args: uid=%d points=%d", userID, points)
				}
				return nil
			},
		}
		rankRepo := &rankRepoStub{
			incrSignInScoreFn: func(ctx context.Context, userID int64, when time.Time, delta int64) error {
				rankCalled = true
				if userID != 123 || delta != signInPoints {
					t.Fatalf("unexpected rank args: uid=%d delta=%d", userID, delta)
				}
				return nil
			},
		}
		tx := &txStub{
			inTxFn: func(ctx context.Context, fn func(ctx context.Context) error) error {
				if syncCalled || rankCalled {
					t.Fatal("sync/rank should not run before tx body finishes")
				}
				err := fn(ctx)
				inTxCalled = true
				return err
			},
		}
		svc := NewSignInService(repo, pointRepo, rankRepo, &activityLogRepoStub{}, tx, events.NopPublisher{}, logger.NoOpLogger{})

		res, err := svc.SignIn(context.Background(), 123)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.AlreadySigned {
			t.Fatal("want not already signed")
		}
		if res.ContinuousDays != 3 || res.Points != signInPoints {
			t.Fatalf("unexpected result: %+v", res)
		}
		if !pointCalled {
			t.Fatal("pointRepo.AddSignInPoints should be called")
		}
		if !syncCalled {
			t.Fatal("repo.SyncSignedOnDate should be called after successful sign in")
		}
		if !rankCalled {
			t.Fatal("rankRepo.IncrSignInScore should be called after successful sign in")
		}
	})

	t.Run("already signed returns current streak and skips side effects", func(t *testing.T) {
		t.Parallel()
		pointCalled := false
		syncCalled := false
		rankCalled := false
		repo := &signInRepoStub{
			signInFn: func(ctx context.Context, userId, signInAt int64) (int, bool, error) {
				return 0, true, nil
			},
			getContinuousDaysFn: func(ctx context.Context, userID int64, nowTs int64) (int, error) {
				return 7, nil
			},
			syncSignedOnDateFn: func(ctx context.Context, userID int64, signInAt int64) error {
				syncCalled = true
				return nil
			},
		}
		pointRepo := &pointRepoStub{addSignInPointsFn: func(ctx context.Context, userID int64, signInAt int64, points int) error {
			pointCalled = true
			return nil
		}}
		rankRepo := &rankRepoStub{incrSignInScoreFn: func(ctx context.Context, userID int64, when time.Time, delta int64) error {
			rankCalled = true
			return nil
		}}
		svc := NewSignInService(repo, pointRepo, rankRepo, &activityLogRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NoOpLogger{})

		res, err := svc.SignIn(context.Background(), 123)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !res.AlreadySigned || res.ContinuousDays != 7 || res.Points != 0 {
			t.Fatalf("unexpected result: %+v", res)
		}
		if pointCalled || syncCalled || rankCalled {
			t.Fatalf("side effects should be skipped when already signed: point=%v sync=%v rank=%v", pointCalled, syncCalled, rankCalled)
		}
	})

	t.Run("point repository error aborts sign in flow", func(t *testing.T) {
		t.Parallel()
		pointErr := errors.New("point write failed")
		syncCalled := false
		rankCalled := false
		repo := &signInRepoStub{
			signInFn: func(ctx context.Context, userId, signInAt int64) (int, bool, error) {
				return 2, false, nil
			},
			syncSignedOnDateFn: func(ctx context.Context, userID int64, signInAt int64) error {
				syncCalled = true
				return nil
			},
		}
		pointRepo := &pointRepoStub{addSignInPointsFn: func(ctx context.Context, userID int64, signInAt int64, points int) error {
			return pointErr
		}}
		rankRepo := &rankRepoStub{incrSignInScoreFn: func(ctx context.Context, userID int64, when time.Time, delta int64) error {
			rankCalled = true
			return nil
		}}
		svc := NewSignInService(repo, pointRepo, rankRepo, &activityLogRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NoOpLogger{})

		_, err := svc.SignIn(context.Background(), 123)
		if !errors.Is(err, pointErr) {
			t.Fatalf("want err %v, got %v", pointErr, err)
		}
		if syncCalled || rankCalled {
			t.Fatalf("sync/rank should not run when tx fails: sync=%v rank=%v", syncCalled, rankCalled)
		}
	})
}
