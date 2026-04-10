package service

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"user-center/internal/domain"
	"user-center/internal/pkg/biztime"
	"user-center/internal/repository"
)

type RankUser struct {
	UserID      int64  `json:"user_id"`
	DisplayName string `json:"display_name"`
	Score       int64  `json:"score"`
	Rank        int64  `json:"rank"`
}

type RankService interface {
	GetDailyTopN(ctx context.Context, limit int64) ([]RankUser, error)
	GetMonthlyTopN(ctx context.Context, limit int64) ([]RankUser, error)
	GetDailyMe(ctx context.Context, userID int64) (RankUser, error)
	GetMonthlyMe(ctx context.Context, userID int64) (RankUser, error)
}

type RankServiceImpl struct {
	rankRepo repository.RankRepository
	userRepo repository.UserRepository
}

func NewRankServiceImpl(rankRepo repository.RankRepository, userRepo repository.UserRepository) *RankServiceImpl {
	return &RankServiceImpl{
		rankRepo: rankRepo,
		userRepo: userRepo,
	}
}

func (s *RankServiceImpl) GetDailyTopN(ctx context.Context, limit int64) ([]RankUser, error) {
	today := currentBusinessDate()
	items, err := s.rankRepo.TopNDaily(ctx, today, limit)
	if err != nil {
		return nil, err
	}
	res := s.enrichRankUser(ctx, items)
	for i := range res {
		res[i].Score = signInPoints
	}
	return res, nil
}

func (s *RankServiceImpl) GetMonthlyTopN(ctx context.Context, limit int64) ([]RankUser, error) {
	today := currentBusinessDate()
	items, err := s.rankRepo.TopNMonthly(ctx, today.Year(), today.Month(), limit)
	if err != nil {
		return nil, err
	}
	res := s.enrichRankUser(ctx, items)
	for i := range res {
		res[i].Score = int64(items[i].Score)
	}
	return res, nil
}

func (s *RankServiceImpl) GetDailyMe(ctx context.Context, userID int64) (RankUser, error) {
	today := currentBusinessDate()
	rank, _, found, err := s.rankRepo.GetDailyRank(ctx, userID, today)
	if err != nil {
		return RankUser{}, err
	}
	if !found {
		res := s.enrichOne(ctx, userID, 0)
		return res, nil
	}
	res := s.enrichOne(ctx, userID, rank)
	res.Score = signInPoints
	return res, nil
}

func (s *RankServiceImpl) GetMonthlyMe(ctx context.Context, userID int64) (RankUser, error) {
	today := currentBusinessDate()
	rank, score, found, err := s.rankRepo.GetMonthlyRank(ctx, userID, today.Year(), today.Month())
	if err != nil {
		return RankUser{}, err
	}
	if !found {
		res := s.enrichOne(ctx, userID, 0)
		return res, nil
	}
	res := s.enrichOne(ctx, userID, rank)
	res.Score = int64(score)
	return res, nil
}

func (s *RankServiceImpl) enrichRankUser(ctx context.Context, items []repository.RankItem) []RankUser {
	res := make([]RankUser, 0, len(items))
	for idx, item := range items {
		res = append(res, s.enrichOne(ctx, item.UserID, int64(idx+1)))
	}
	return res
}

func (s *RankServiceImpl) enrichOne(ctx context.Context, userID int64, rank int64) RankUser {
	res := RankUser{UserID: userID,
		DisplayName: "用户" + strconv.FormatInt(userID, 10),
		Rank:        rank}
	user, err := s.userRepo.FindByID(ctx, userID)
	if err == nil {
		res.DisplayName = buildDisplayName(user)
	}
	return res
}

func buildDisplayName(user domain.User) string {
	return fmt.Sprintf("用户：%s", strconv.FormatInt(user.Id, 10))
}

func currentBusinessDate() time.Time {
	return biztime.StartOfBizDay(biztime.NowMillis())
}
