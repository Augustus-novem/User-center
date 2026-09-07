package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"user-center/internal/domain"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

var (
	ErrFollowSelf       = errors.New("不能关注自己")
	ErrFolloweeNotFound = errors.New("用户不存在")
	ErrInvalidFollowID  = errors.New("用户 ID 无效")
	ErrInvalidCursor    = errors.New("游标无效")
)

const (
	defaultFollowPageSize = 20
	maxFollowPageSize     = 50
)

type FollowService interface {
	Follow(ctx context.Context, followerID, followeeID int64) error
	Unfollow(ctx context.Context, followerID, followeeID int64) error
	ListFollowers(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error)
	ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error)
}

type FollowServiceImpl struct {
	followRepo repository.FollowRepository
	userRepo   repository.UserRepository
	logger     logger.Logger
}

func NewFollowServiceImpl(followRepo repository.FollowRepository, userRepo repository.UserRepository, l logger.Logger) *FollowServiceImpl {
	return &FollowServiceImpl{
		followRepo: followRepo,
		userRepo:   userRepo,
		logger:     l,
	}
}

func (s *FollowServiceImpl) Follow(ctx context.Context, followerID, followeeID int64) error {
	if followerID <= 0 || followeeID <= 0 {
		return ErrInvalidFollowID
	}
	if followerID == followeeID {
		return ErrFollowSelf
	}
	if _, err := s.userRepo.FindByID(ctx, followeeID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return ErrFolloweeNotFound
		}
		return err
	}
	err := s.followRepo.Create(ctx, followerID, followeeID)
	if errors.Is(err, repository.ErrFollowDuplicate) {
		s.logger.Info("follow already exists",
			logger.Field{Key: "user_id", Value: followerID},
			logger.Field{Key: "followee_id", Value: followeeID},
		)
		return nil
	}
	if err != nil {
		s.logger.Error("follow failed",
			logger.Field{Key: "user_id", Value: followerID},
			logger.Field{Key: "followee_id", Value: followeeID},
			logger.Error(err),
		)
		return err
	}
	return nil
}

func (s *FollowServiceImpl) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	if followerID <= 0 || followeeID <= 0 {
		return ErrInvalidFollowID
	}
	if err := s.followRepo.Delete(ctx, followerID, followeeID); err != nil {
		s.logger.Error("unfollow failed",
			logger.Field{Key: "user_id", Value: followerID},
			logger.Field{Key: "followee_id", Value: followeeID},
			logger.Error(err),
		)
		return err
	}
	return nil
}

func (s *FollowServiceImpl) ListFollowers(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error) {
	return s.list(ctx, userID, cursor, limit, true)
}

func (s *FollowServiceImpl) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error) {
	return s.list(ctx, userID, cursor, limit, false)
}

func (s *FollowServiceImpl) list(ctx context.Context, userID int64, rawCursor string, limit int, followers bool) (domain.FollowPage, error) {
	if userID <= 0 {
		return domain.FollowPage{}, ErrInvalidFollowID
	}
	cur, err := parseFollowCursor(rawCursor)
	if err != nil {
		return domain.FollowPage{}, err
	}
	limit = normalizeFollowLimit(limit)
	var rows []domain.UserRelation
	if followers {
		rows, err = s.followRepo.ListFollowers(ctx, userID, cur, limit+1)
	} else {
		rows, err = s.followRepo.ListFollowing(ctx, userID, cur, limit+1)
	}
	if err != nil {
		return domain.FollowPage{}, err
	}
	page := domain.FollowPage{Items: make([]domain.FollowListItem, 0, len(rows))}
	if len(rows) > limit {
		page.HasMore = true
		rows = rows[:limit]
	}
	for _, row := range rows {
		item := domain.FollowListItem{CreatedAt: row.CreatedAt}
		if followers {
			item.UserID = row.FollowerID
		} else {
			item.UserID = row.FolloweeID
		}
		page.Items = append(page.Items, item)
	}
	if page.HasMore && len(rows) > 0 {
		last := rows[len(rows)-1]
		page.NextCursor = encodeFollowCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

func normalizeFollowLimit(limit int) int {
	if limit <= 0 {
		return defaultFollowPageSize
	}
	if limit > maxFollowPageSize {
		return maxFollowPageSize
	}
	return limit
}

func encodeFollowCursor(createdAt, id int64) string {
	return strconv.FormatInt(createdAt, 10) + "_" + strconv.FormatInt(id, 10)
}

func parseFollowCursor(raw string) (*domain.FollowCursor, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "_")
	if len(parts) != 2 {
		return nil, ErrInvalidCursor
	}
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || createdAt <= 0 {
		return nil, ErrInvalidCursor
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return nil, ErrInvalidCursor
	}
	return &domain.FollowCursor{CreatedAt: createdAt, ID: id}, nil
}
