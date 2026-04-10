package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"user-center/internal/pkg/biztime"
	"user-center/internal/repository/dao"
)

type signInDAOStub struct {
	createRecordFn      func(ctx context.Context, record dao.UserSignInRecordOfDB) error
	getStatFn           func(ctx context.Context, userID int64) (dao.UserSignInStatOfDB, error)
	createStatFn        func(ctx context.Context, stat dao.UserSignInStatOfDB) error
	updateStatFn        func(ctx context.Context, stat dao.UserSignInStatOfDB) error
	hasSignedOnBizDayFn func(ctx context.Context, userID int64, bizDay int) (bool, error)
	listSignedBizDaysFn func(ctx context.Context, userId int64, monthStartMs, nextMonthStartMs int64) ([]int, error)
}

func (s *signInDAOStub) CreateRecord(ctx context.Context, record dao.UserSignInRecordOfDB) error {
	if s.createRecordFn == nil {
		return nil
	}
	return s.createRecordFn(ctx, record)
}

func (s *signInDAOStub) GetStat(ctx context.Context, userID int64) (dao.UserSignInStatOfDB, error) {
	if s.getStatFn == nil {
		return dao.UserSignInStatOfDB{}, nil
	}
	return s.getStatFn(ctx, userID)
}

func (s *signInDAOStub) CreateStat(ctx context.Context, stat dao.UserSignInStatOfDB) error {
	if s.createStatFn == nil {
		return nil
	}
	return s.createStatFn(ctx, stat)
}

func (s *signInDAOStub) UpdateStat(ctx context.Context, stat dao.UserSignInStatOfDB) error {
	if s.updateStatFn == nil {
		return nil
	}
	return s.updateStatFn(ctx, stat)
}

func (s *signInDAOStub) HasSignedOnBizDay(ctx context.Context, userID int64, bizDay int) (bool, error) {
	if s.hasSignedOnBizDayFn == nil {
		return false, nil
	}
	return s.hasSignedOnBizDayFn(ctx, userID, bizDay)
}

func (s *signInDAOStub) ListSignedBizDaysInMonth(ctx context.Context, userId int64, monthStartMs, nextMonthStartMs int64) ([]int, error) {
	if s.listSignedBizDaysFn == nil {
		return nil, nil
	}
	return s.listSignedBizDaysFn(ctx, userId, monthStartMs, nextMonthStartMs)
}

type signInCacheStub struct {
	setSignedFn           func(ctx context.Context, uid int64, signDate time.Time) error
	isSignedOnDateFn      func(ctx context.Context, userID int64, signDate time.Time) (bool, error)
	getMonthSignedDaysFn  func(ctx context.Context, userID int64, year int, month time.Month) ([]int, bool, error)
	batchSetMonthSignedFn func(ctx context.Context, userID int64, year int, month time.Month, days []int) error
}

func (s *signInCacheStub) SetSigned(ctx context.Context, uid int64, signDate time.Time) error {
	if s.setSignedFn == nil {
		return nil
	}
	return s.setSignedFn(ctx, uid, signDate)
}

func (s *signInCacheStub) IsSignedOnDate(ctx context.Context, userID int64, signDate time.Time) (bool, error) {
	if s.isSignedOnDateFn == nil {
		return false, nil
	}
	return s.isSignedOnDateFn(ctx, userID, signDate)
}

func (s *signInCacheStub) GetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month) ([]int, bool, error) {
	if s.getMonthSignedDaysFn == nil {
		return nil, false, nil
	}
	return s.getMonthSignedDaysFn(ctx, userID, year, month)
}

func (s *signInCacheStub) BatchSetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month, days []int) error {
	if s.batchSetMonthSignedFn == nil {
		return nil
	}
	return s.batchSetMonthSignedFn(ctx, userID, year, month, days)
}

func TestSignInRepositoryImpl_GetContinuousDays(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 4, 10, 10, 0, 0, 0, biztime.Location())
	nowMs := now.UnixMilli()
	yesterdayMs := now.AddDate(0, 0, -1).UnixMilli()
	threeDaysAgoMs := now.AddDate(0, 0, -3).UnixMilli()

	tests := []struct {
		name    string
		stat    dao.UserSignInStatOfDB
		statErr error
		want    int
		wantErr error
	}{
		{name: "not found returns zero", statErr: dao.ErrDataNotFound, want: 0},
		{name: "signed today keeps streak", stat: dao.UserSignInStatOfDB{ContinuousDays: 5, LastSignAt: nowMs}, want: 5},
		{name: "signed yesterday keeps streak", stat: dao.UserSignInStatOfDB{ContinuousDays: 4, LastSignAt: yesterdayMs}, want: 4},
		{name: "older sign resets streak", stat: dao.UserSignInStatOfDB{ContinuousDays: 9, LastSignAt: threeDaysAgoMs}, want: 0},
		{name: "db error bubbles up", statErr: errors.New("db down"), wantErr: errors.New("db down")},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo := NewSignInRepositoryImpl(&signInDAOStub{
				getStatFn: func(ctx context.Context, userID int64) (dao.UserSignInStatOfDB, error) {
					return tc.stat, tc.statErr
				},
			}, &signInCacheStub{})
			got, err := repo.GetContinuousDays(context.Background(), 1, nowMs)
			if tc.wantErr != nil {
				if err == nil || err.Error() != tc.wantErr.Error() {
					t.Fatalf("want err %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("want %d, got %d", tc.want, got)
			}
		})
	}
}

func TestSignInRepositoryImpl_IsSignedOnDate(t *testing.T) {
	t.Parallel()

	ts := time.Date(2026, 4, 10, 15, 0, 0, 0, biztime.Location()).UnixMilli()

	t.Run("cache hit returns immediately", func(t *testing.T) {
		t.Parallel()
		daoCalled := false
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			hasSignedOnBizDayFn: func(ctx context.Context, userID int64, bizDay int) (bool, error) {
				daoCalled = true
				return false, nil
			},
		}, &signInCacheStub{
			isSignedOnDateFn: func(ctx context.Context, userID int64, signDate time.Time) (bool, error) {
				return true, nil
			},
		})
		signed, err := repo.IsSignedOnDate(context.Background(), 1, ts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !signed {
			t.Fatal("want signed=true")
		}
		if daoCalled {
			t.Fatal("dao should not be called on cache hit")
		}
	})

	t.Run("cache miss falls back to dao and backfills cache", func(t *testing.T) {
		t.Parallel()
		cacheWritten := false
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			hasSignedOnBizDayFn: func(ctx context.Context, userID int64, bizDay int) (bool, error) {
				if bizDay != 20260410 {
					t.Fatalf("unexpected bizDay: %d", bizDay)
				}
				return true, nil
			},
		}, &signInCacheStub{
			isSignedOnDateFn: func(ctx context.Context, userID int64, signDate time.Time) (bool, error) {
				return false, errors.New("cache miss")
			},
			setSignedFn: func(ctx context.Context, uid int64, signDate time.Time) error {
				cacheWritten = true
				if signDate.Day() != 10 {
					t.Fatalf("unexpected sign date: %v", signDate)
				}
				return nil
			},
		})
		signed, err := repo.IsSignedOnDate(context.Background(), 1, ts)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !signed {
			t.Fatal("want signed=true")
		}
		if !cacheWritten {
			t.Fatal("cache.SetSigned should be called after dao confirms sign-in")
		}
	})
}

func TestSignInRepositoryImpl_SignIn(t *testing.T) {
	t.Parallel()

	t.Run("duplicate sign-in returns already signed", func(t *testing.T) {
		t.Parallel()
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			createRecordFn: func(ctx context.Context, record dao.UserSignInRecordOfDB) error {
				return dao.ErrSignInDuplicate
			},
		}, &signInCacheStub{})
		continuous, alreadySigned, err := repo.SignIn(context.Background(), 8, time.Date(2026, 4, 10, 10, 0, 0, 0, biztime.Location()).UnixMilli())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !alreadySigned || continuous != 0 {
			t.Fatalf("unexpected result: continuous=%d already=%v", continuous, alreadySigned)
		}
	})

	t.Run("first sign-in creates new stat", func(t *testing.T) {
		t.Parallel()
		var created dao.UserSignInStatOfDB
		signInAt := time.Date(2026, 4, 10, 10, 0, 0, 0, biztime.Location()).UnixMilli()
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			createRecordFn: func(ctx context.Context, record dao.UserSignInRecordOfDB) error {
				if record.UserId != 9 || record.BizDay != 20260410 || record.SignInAt != signInAt {
					t.Fatalf("unexpected record: %+v", record)
				}
				return nil
			},
			getStatFn: func(ctx context.Context, userID int64) (dao.UserSignInStatOfDB, error) {
				return dao.UserSignInStatOfDB{}, dao.ErrDataNotFound
			},
			createStatFn: func(ctx context.Context, stat dao.UserSignInStatOfDB) error {
				created = stat
				return nil
			},
		}, &signInCacheStub{})
		continuous, alreadySigned, err := repo.SignIn(context.Background(), 9, signInAt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if alreadySigned || continuous != 1 {
			t.Fatalf("unexpected result: continuous=%d already=%v", continuous, alreadySigned)
		}
		if created.UserId != 9 || created.ContinuousDays != 1 || created.TotalDays != 1 || created.LastSignAt != signInAt {
			t.Fatalf("unexpected created stat: %+v", created)
		}
	})

	t.Run("consecutive sign-in increments streak", func(t *testing.T) {
		t.Parallel()
		var updated dao.UserSignInStatOfDB
		now := time.Date(2026, 4, 10, 10, 0, 0, 0, biztime.Location())
		yesterday := now.AddDate(0, 0, -1).UnixMilli()
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			createRecordFn: func(ctx context.Context, record dao.UserSignInRecordOfDB) error { return nil },
			getStatFn: func(ctx context.Context, userID int64) (dao.UserSignInStatOfDB, error) {
				return dao.UserSignInStatOfDB{Id: 3, UserId: 11, ContinuousDays: 2, TotalDays: 5, LastSignAt: yesterday}, nil
			},
			updateStatFn: func(ctx context.Context, stat dao.UserSignInStatOfDB) error {
				updated = stat
				return nil
			},
		}, &signInCacheStub{})
		continuous, alreadySigned, err := repo.SignIn(context.Background(), 11, now.UnixMilli())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if alreadySigned || continuous != 3 {
			t.Fatalf("unexpected result: continuous=%d already=%v", continuous, alreadySigned)
		}
		if updated.ContinuousDays != 3 || updated.TotalDays != 6 {
			t.Fatalf("unexpected updated stat: %+v", updated)
		}
	})

	t.Run("non-consecutive sign-in resets streak to one", func(t *testing.T) {
		t.Parallel()
		var updated dao.UserSignInStatOfDB
		now := time.Date(2026, 4, 10, 10, 0, 0, 0, biztime.Location())
		old := now.AddDate(0, 0, -3).UnixMilli()
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			createRecordFn: func(ctx context.Context, record dao.UserSignInRecordOfDB) error { return nil },
			getStatFn: func(ctx context.Context, userID int64) (dao.UserSignInStatOfDB, error) {
				return dao.UserSignInStatOfDB{Id: 4, UserId: 12, ContinuousDays: 9, TotalDays: 12, LastSignAt: old}, nil
			},
			updateStatFn: func(ctx context.Context, stat dao.UserSignInStatOfDB) error {
				updated = stat
				return nil
			},
		}, &signInCacheStub{})
		continuous, alreadySigned, err := repo.SignIn(context.Background(), 12, now.UnixMilli())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if alreadySigned || continuous != 1 || updated.ContinuousDays != 1 {
			t.Fatalf("unexpected updated stat: %+v", updated)
		}
	})
}

func TestSignInRepositoryImpl_GetMonthSignedDays(t *testing.T) {
	t.Parallel()

	t.Run("cache hit returns directly", func(t *testing.T) {
		t.Parallel()
		daoCalled := false
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			listSignedBizDaysFn: func(ctx context.Context, userId int64, monthStartMs, nextMonthStartMs int64) ([]int, error) {
				daoCalled = true
				return nil, nil
			},
		}, &signInCacheStub{
			getMonthSignedDaysFn: func(ctx context.Context, userID int64, year int, month time.Month) ([]int, bool, error) {
				return []int{1, 5, 9}, true, nil
			},
		})
		days, err := repo.GetMonthSignedDays(context.Background(), 1, 2026, time.April)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if daoCalled {
			t.Fatal("dao should not be called on cache hit")
		}
		if len(days) != 3 || days[0] != 1 || days[1] != 5 || days[2] != 9 {
			t.Fatalf("unexpected days: %v", days)
		}
	})

	t.Run("cache miss queries dao and backfills cache", func(t *testing.T) {
		t.Parallel()
		backfilled := false
		repo := NewSignInRepositoryImpl(&signInDAOStub{
			listSignedBizDaysFn: func(ctx context.Context, userId int64, monthStartMs, nextMonthStartMs int64) ([]int, error) {
				return []int{20260401, 20260403, 20260410}, nil
			},
		}, &signInCacheStub{
			getMonthSignedDaysFn: func(ctx context.Context, userID int64, year int, month time.Month) ([]int, bool, error) {
				return nil, false, errors.New("cache miss")
			},
			batchSetMonthSignedFn: func(ctx context.Context, userID int64, year int, month time.Month, days []int) error {
				backfilled = true
				if year != 2026 || month != time.April || len(days) != 3 || days[2] != 10 {
					t.Fatalf("unexpected backfill args: year=%d month=%d days=%v", year, month, days)
				}
				return nil
			},
		})
		days, err := repo.GetMonthSignedDays(context.Background(), 1, 2026, time.April)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !backfilled {
			t.Fatal("cache should be backfilled on dao hit")
		}
		if len(days) != 3 || days[0] != 1 || days[1] != 3 || days[2] != 10 {
			t.Fatalf("unexpected days: %v", days)
		}
	})
}

func TestSignInRepositoryImpl_SyncSignedOnDate(t *testing.T) {
	t.Parallel()

	signInAt := time.Date(2026, 4, 10, 13, 12, 0, 0, biztime.Location()).UnixMilli()
	called := false
	repo := NewSignInRepositoryImpl(&signInDAOStub{}, &signInCacheStub{
		setSignedFn: func(ctx context.Context, uid int64, signDate time.Time) error {
			called = true
			if uid != 99 || signDate.Hour() != 0 || signDate.Day() != 10 {
				t.Fatalf("unexpected args: uid=%d signDate=%v", uid, signDate)
			}
			return nil
		},
	})
	if err := repo.SyncSignedOnDate(context.Background(), 99, signInAt); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("cache.SetSigned should be called")
	}
}
