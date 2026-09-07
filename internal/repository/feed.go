package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

type FeedRepository interface {
	ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error)
}

type FeedRepositoryImpl struct {
	dao dao.FeedDAO
}

func NewFeedRepositoryImpl(d dao.FeedDAO) *FeedRepositoryImpl {
	return &FeedRepositoryImpl{dao: d}
}

func (r *FeedRepositoryImpl) ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
	var daoCursor *dao.NoteCursor
	if cursor != nil {
		daoCursor = &dao.NoteCursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID}
	}
	rows, err := r.dao.ListFollowingNotes(ctx, followerID, daoCursor, limit)
	if err != nil {
		return nil, err
	}
	res := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		res = append(res, toDomainNote(row, nil))
	}
	return res, nil
}
