package service

import (
	"context"
	"time"
	"user-center/internal/events"
	"user-center/internal/pkg/biztime"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

const signInPoints = 5

type SignInResult struct {
	AlreadySigned  bool
	ContinuousDays int
	Points         int
}

type SignInService interface {
	SignIn(ctx context.Context, userID int64) (SignInResult, error)
	GetTodayStatus(ctx context.Context, userID int64) (bool, error)
	GetMonthRecords(ctx context.Context, userID int64, year, month int) ([]int, error)
	GetStreak(ctx context.Context, userID int64) (int, error)
}

type SignInServiceImpl struct {
	repo            repository.SignInRepository
	pointRepo       repository.PointRepository
	rankRepo        repository.RankRepository
	activityLogRepo repository.ActivityLogRepository
	tx              repository.Transaction
	publisher       events.Publisher
	logger          logger.Logger
}

func NewSignInService(repo repository.SignInRepository,
	pointRepo repository.PointRepository,
	rankRepo repository.RankRepository,
	activityLogRepo repository.ActivityLogRepository,
	tx repository.Transaction,
	publisher events.Publisher,
	l logger.Logger) *SignInServiceImpl {
	return &SignInServiceImpl{
		repo:            repo,
		pointRepo:       pointRepo,
		rankRepo:        rankRepo,
		activityLogRepo: activityLogRepo,
		tx:              tx,
		publisher:       publisher,
		logger:          l,
	}
}

func (s *SignInServiceImpl) SignIn(ctx context.Context,
	userID int64) (SignInResult, error) {
	now := biztime.NowMillis()
	bizDay := biztime.BizDayString(now)
	var res SignInResult
	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		streak, already, err := s.repo.SignIn(txCtx, userID, now)
		if err != nil {
			return err
		}
		if already {
			res.AlreadySigned = true
			res.ContinuousDays, err = s.repo.GetContinuousDays(txCtx, userID, now)
			return err
		}
		res.ContinuousDays = streak
		res.Points = signInPoints
		if err = s.pointRepo.AddSignInPoints(txCtx, userID, now, signInPoints); err != nil {
			return err
		}
		if s.publisher != nil && s.publisher.IsEnabled() {
			evt := events.NewUserCheckInEvent(userID, bizDay, signInPoints)
			if err = s.publisher.Publish(txCtx, events.TopicUserActivity,
				events.UserIDKey(userID), evt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return SignInResult{}, err
	}
	if res.AlreadySigned {
		return res, nil
	}
	if err = s.repo.SyncSignedOnDate(ctx, userID, now); err != nil {
		s.logger.Warn("同步签到缓存失败",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "error", Value: err},
		)
	}
	if s.publisher != nil && s.publisher.IsEnabled() {
		return res, nil
	}
	s.logger.Warn("Kafka 未启用，签到后置动作改为同步执行",
		logger.Field{Key: "user_id", Value: userID},
		logger.Field{Key: "biz_day", Value: bizDay},
	)
	if err = s.rankRepo.IncrSignInScore(ctx, userID, biztime.ToTime(now), signInPoints); err != nil {
		s.logger.Error("同步更新签到排行榜失败",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "error", Value: err},
		)
	}
	if err = s.activityLogRepo.Append(ctx, repository.ActivityLogEntry{
		UserID:     userID,
		Action:     events.ActionCheckIn,
		BizID:      bizDay,
		Points:     signInPoints,
		OccurredAt: now,
	}); err != nil {
		s.logger.Error("同步写入签到行为日志失败",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "error", Value: err},
		)
	}
	return res, nil
}

func (s *SignInServiceImpl) GetTodayStatus(ctx context.Context,
	userID int64) (bool, error) {
	return s.repo.IsSignedOnDate(ctx, userID, biztime.NowMillis())
}

func (s *SignInServiceImpl) GetMonthRecords(ctx context.Context,
	userID int64, year, month int) ([]int, error) {
	return s.repo.GetMonthSignedDays(ctx, userID, year, time.Month(month))
}

func (s *SignInServiceImpl) GetStreak(ctx context.Context, userID int64) (int, error) {
	return s.repo.GetContinuousDays(ctx, userID, biztime.NowMillis())
}
