package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var ErrFollowDuplicate = dao.ErrFollowDuplicate

type FollowRepository interface {
	Create(ctx context.Context, followerID, followeeID int64) error
	Delete(ctx context.Context, followerID, followeeID int64) error
	ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error)
	ListFollowers(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error)
}

type FollowRepositoryImpl struct {
	dao dao.FollowDAO
}

func NewFollowRepositoryImpl(d dao.FollowDAO) *FollowRepositoryImpl {
	return &FollowRepositoryImpl{dao: d}
}

func (r *FollowRepositoryImpl) Create(ctx context.Context, followerID, followeeID int64) error {
	return r.dao.Insert(ctx, dao.UserRelationOfDB{
		FollowerId: followerID,
		FolloweeId: followeeID,
	})
}

func (r *FollowRepositoryImpl) Delete(ctx context.Context, followerID, followeeID int64) error {
	return r.dao.Delete(ctx, followerID, followeeID)
}

func (r *FollowRepositoryImpl) ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
	rows, err := r.dao.ListFollowing(ctx, followerID, toDAOFollowCursor(cursor), limit)
	if err != nil {
		return nil, err
	}
	return toDomainRelations(rows), nil
}

func (r *FollowRepositoryImpl) ListFollowers(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
	rows, err := r.dao.ListFollowers(ctx, followeeID, toDAOFollowCursor(cursor), limit)
	if err != nil {
		return nil, err
	}
	return toDomainRelations(rows), nil
}

func toDAOFollowCursor(cursor *domain.FollowCursor) *dao.FollowCursor {
	if cursor == nil {
		return nil
	}
	return &dao.FollowCursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID}
}

func toDomainRelations(rows []dao.UserRelationOfDB) []domain.UserRelation {
	res := make([]domain.UserRelation, 0, len(rows))
	for _, row := range rows {
		res = append(res, domain.UserRelation{
			ID:         row.Id,
			FollowerID: row.FollowerId,
			FolloweeID: row.FolloweeId,
			CreatedAt:  row.CreatedAt,
		})
	}
	return res
}
