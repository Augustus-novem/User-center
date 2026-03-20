package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var ErrUserDuplicateEmail = dao.ErrUserDuplicateEmail

type UserRepo struct {
	dao *dao.UserDao
}

func NewUserRepo(dao *dao.UserDao) *UserRepo {
	return &UserRepo{dao}
}

func (u *UserRepo) Create(c context.Context, user domain.User) error {

	return u.dao.Insert(c, dao.UserOfDB{
		Email:    user.Email,
		Password: user.Password,
	})
}
