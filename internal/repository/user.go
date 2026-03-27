package repository

import (
	"context"
	"database/sql"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
)

var (
	ErrUserDuplicate = dao.ErrUserDuplicate
	ErrUserNotFound  = dao.ErrDataNotFound
)

type UserRepository struct {
	dao   *dao.UserDAO
	cache *cache.UserCache
}

func NewUserRepo(dao *dao.UserDAO, cmd *cache.UserCache) *UserRepository {
	return &UserRepository{
		dao:   dao,
		cache: cmd,
	}
}

func (ur *UserRepository) Create(c context.Context, user domain.User) error {

	return ur.dao.Insert(c, dao.UserOfDB{
		Email: sql.NullString{
			String: user.Email,
			Valid:  user.Email != "",
		},
		Phone: sql.NullString{
			String: user.Phone,
			Valid:  user.Phone != "",
		},
		Password: user.Password,
	})
}

func (ur *UserRepository) FindByEmail(ctx context.Context,
	email string) (domain.User, error) {
	user, err := ur.dao.FindByEmail(ctx, email)
	return ur.entityToDomain(user), err
}

func (ur *UserRepository) FindByID(ctx context.Context,
	ID int64) (domain.User, error) {
	user, err := ur.cache.Get(ctx, ID)
	if err == nil {
		return user, nil
	}
	ue, err := ur.dao.FindById(ctx, ID)
	if err != nil {
		return domain.User{}, err
	}
	user = ur.entityToDomain(ue)
	_ = ur.cache.Set(ctx, user)
	return user, nil
}

func (ur *UserRepository) FindByPhone(ctx context.Context, phone string) (domain.User, error) {
	user, err := ur.dao.FindByPhone(ctx, phone)
	return ur.entityToDomain(user), err
}

func (ur *UserRepository) entityToDomain(ue dao.UserOfDB) domain.User {
	return domain.User{
		Id:       ue.Id,
		Email:    ue.Email.String,
		Password: ue.Password,
		Phone:    ue.Phone.String,
	}
}
