package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var ErrUserDuplicateEmail = dao.ErrUserDuplicateEmail
var ErrUserNotFound = dao.ErrDataNotFound

type UserRepo struct {
	dao *dao.UserDao
}

func NewUserRepo(dao *dao.UserDao) *UserRepo {
	return &UserRepo{dao}
}

func (ur *UserRepo) Create(c context.Context, user domain.User) error {

	return ur.dao.Insert(c, dao.UserOfDB{
		Email:    user.Email,
		Password: user.Password,
	})
}

func (ur *UserRepo) FindByEmail(ctx context.Context,
	email string) (domain.User, error) {
	user, err := ur.dao.FindByEmail(ctx, email)
	return domain.User{
		Id:       user.Id,
		Email:    user.Email,
		Password: user.Password,
	}, err
}

func (ur *UserRepo) FindByID(ctx context.Context,
	ID int64) (domain.User, error) {
	user, err := ur.dao.FindByID(ctx, ID)
	return domain.User{
		Id:       user.Id,
		Email:    user.Email,
		Password: user.Password,
	}, err
}
