package repository

import (
	"context"
	"errors"
	"user-center/internal/pkg/biztime"
	"user-center/internal/repository/dao"
)

const BizTypeSignIn = "signIn"

type PointRepository interface {
	AddSignInPoints(ctx context.Context, userID int64, signInAt int64, points int) error
}

type PointRepositoryImpl struct {
	dao dao.PointDAO
}

func NewPointRepositoryImpl(dao dao.PointDAO) *PointRepositoryImpl {
	return &PointRepositoryImpl{
		dao: dao,
	}
}

func (r PointRepositoryImpl) AddSignInPoints(ctx context.Context,
	userID int64, signInAt int64, points int) error {
	err := r.dao.CreateRecord(ctx, dao.UserPointRecordOfDB{
		UserId:  userID,
		BizType: BizTypeSignIn,
		BizId:   biztime.BizDayString(signInAt),
		Points:  points,
	})
	if errors.Is(err, dao.ErrPointRecordDuplicate) {
		return nil
	}
	return err
}
