package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
)

var ErrUserDuplicateEmail = dao.ErrUserDuplicateEmail
var ErrUserNotFound = dao.ErrDataNotFound

type UserRepo struct {
	dao   *dao.UserDao
	cache *cache.UserCache
}

func NewUserRepo(dao *dao.UserDao, cmd *cache.UserCache) *UserRepo {
	return &UserRepo{
		dao:   dao,
		cache: cmd,
	}
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
	user, err := ur.cache.Get(ctx, ID)
	if err == nil {
		return user, nil
	}
	ue, err := ur.dao.FindByID(ctx, ID)
	if err != nil {
		return domain.User{}, err
	}
	user = domain.User{
		Id:       ue.Id,
		Email:    ue.Email,
		Password: ue.Password,
	}
	_ = ur.cache.Set(ctx, user)
	return user, nil
}
