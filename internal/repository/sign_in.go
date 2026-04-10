package repository

import (
	"context"
	"errors"
	"time"
	"user-center/internal/pkg/biztime"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
)

type SignInRepository interface {
	SignIn(ctx context.Context, userId, signInAt int64) (int, bool, error)
	GetContinuousDays(ctx context.Context, userID int64, nowTs int64) (int, error)
	SyncSignedOnDate(ctx context.Context, userID int64, signInAt int64) error
	IsSignedOnDate(ctx context.Context, userID int64, ts int64) (bool, error)
	GetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month) ([]int, error)
}

type SignInRepositoryImpl struct {
	dao   dao.SignInDAO
	cache cache.SignInCache
}

func NewSignInRepositoryImpl(dao dao.SignInDAO, cache cache.SignInCache) *SignInRepositoryImpl {
	return &SignInRepositoryImpl{
		dao:   dao,
		cache: cache,
	}
}

func (r *SignInRepositoryImpl) SignIn(ctx context.Context, userId, signInAt int64) (int, bool, error) {
	bizDay := biztime.BizDay(signInAt)
	err := r.dao.CreateRecord(ctx, dao.UserSignInRecordOfDB{
		UserId:   userId,
		BizDay:   bizDay,
		SignInAt: signInAt,
	})
	if errors.Is(err, dao.ErrSignInDuplicate) {
		return 0, true, nil
	}
	if err != nil {
		return 0, false, err
	}
	stat, err := r.dao.GetStat(ctx, userId)
	if errors.Is(err, dao.ErrDataNotFound) {
		newStat := dao.UserSignInStatOfDB{
			UserId:         userId,
			ContinuousDays: 1,
			TotalDays:      1,
			LastSignAt:     signInAt,
		}
		if err = r.dao.CreateStat(ctx, newStat); err != nil {
			return 0, false, err
		}
		return 1, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	yesterdayBizDay := biztime.YesterdayBizDay(signInAt)
	continuousDays := 1
	if stat.LastSignAt > 0 && biztime.BizDay(stat.LastSignAt) == yesterdayBizDay {
		continuousDays = stat.ContinuousDays + 1
	}
	stat.ContinuousDays = continuousDays
	stat.TotalDays++
	stat.LastSignAt = signInAt
	if err = r.dao.UpdateStat(ctx, stat); err != nil {
		return 0, false, err
	}
	return stat.ContinuousDays, false, nil
}

func (r *SignInRepositoryImpl) GetContinuousDays(ctx context.Context, userID int64, nowTs int64) (int, error) {
	stat, err := r.dao.GetStat(ctx, userID)
	if errors.Is(err, dao.ErrDataNotFound) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if stat.LastSignAt == 0 {
		return 0, nil
	}
	lastBizDay := biztime.BizDay(stat.LastSignAt)
	todayBizDay := biztime.BizDay(nowTs)
	yesterdayBizDay := biztime.YesterdayBizDay(nowTs)
	if lastBizDay != yesterdayBizDay && lastBizDay != todayBizDay {
		return 0, nil
	}
	return stat.ContinuousDays, nil
}

func (r *SignInRepositoryImpl) IsSignedOnDate(ctx context.Context, userID int64, ts int64) (bool, error) {
	dayStart := biztime.StartOfBizDay(ts)
	signed, err := r.cache.IsSignedOnDate(ctx, userID, dayStart)
	if err == nil && signed {
		return true, nil
	}
	signed, err = r.dao.HasSignedOnBizDay(ctx, userID, biztime.BizDay(ts))
	if err != nil {
		return false, err
	}
	if signed {
		_ = r.cache.SetSigned(ctx, userID, dayStart)
	}
	return signed, nil
}

func (r *SignInRepositoryImpl) GetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month) ([]int, error) {
	days, found, err := r.cache.GetMonthSignedDays(ctx, userID, year, month)
	if err == nil && found {
		return days, nil
	}
	monthStartMs, nextMonthStartMS := biztime.MonthRangeMillis(year, month)
	bizDays, err := r.dao.ListSignedBizDaysInMonth(ctx, userID, monthStartMs, nextMonthStartMS)
	if err != nil {
		return nil, err
	}
	res := make([]int, 0, len(bizDays))
	for _, day := range bizDays {
		res = append(res, biztime.DayOfMonthFromBizDay(day))
	}
	if len(res) > 0 {
		_ = r.cache.BatchSetMonthSignedDays(ctx, userID, year, month, res)
	}
	return res, nil
}

func (r *SignInRepositoryImpl) SyncSignedOnDate(ctx context.Context, userID int64, signInAt int64) error {
	return r.cache.SetSigned(ctx, userID, biztime.StartOfBizDay(signInAt))
}
