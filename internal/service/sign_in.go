package service

import (
	"context"
	"time"
	"user-center/internal/pkg/biztime"
	"user-center/internal/repository"
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
	repo      repository.SignInRepository
	pointRepo repository.PointRepository
	rankRepo  repository.RankRepository
	tx        repository.Transaction
}

func NewSignInService(repo repository.SignInRepository,
	pointRepo repository.PointRepository,
	rankRepo repository.RankRepository,
	tx repository.Transaction) *SignInServiceImpl {
	return &SignInServiceImpl{
		repo:      repo,
		pointRepo: pointRepo,
		rankRepo:  rankRepo,
		tx:        tx,
	}
}

func (s *SignInServiceImpl) SignIn(ctx context.Context,
	userID int64) (SignInResult, error) {
	now := biztime.NowMillis()
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
		return s.pointRepo.AddSignInPoints(txCtx, userID, now, signInPoints)
	})
	if err != nil {
		return SignInResult{}, err
	}
	if !res.AlreadySigned {
		_ = s.repo.SyncSignedOnDate(ctx, userID, now)
		_ = s.rankRepo.IncrSignInScore(ctx, userID, biztime.ToTime(now), signInPoints)
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
