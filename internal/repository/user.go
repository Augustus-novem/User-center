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

type UserRepository interface {
	Create(ctx context.Context, user domain.User) error
	FindByPhone(ctx context.Context, phone string) (domain.User, error)
	FindByID(ctx context.Context, id int64) (domain.User, error)
	FindByEmail(ctx context.Context, email string) (domain.User, error)
}

type CachedUserRepository struct {
	dao   dao.UserDAO
	cache cache.UserCache
}

func NewCachedUserRepository(dao dao.UserDAO, cmd cache.UserCache) *CachedUserRepository {
	return &CachedUserRepository{
		dao:   dao,
		cache: cmd,
	}
}

func (ur *CachedUserRepository) Create(c context.Context, user domain.User) error {

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

func (ur *CachedUserRepository) FindByEmail(ctx context.Context,
	email string) (domain.User, error) {
	user, err := ur.dao.FindByEmail(ctx, email)
	return ur.entityToDomain(user), err
}

func (ur *CachedUserRepository) FindByID(ctx context.Context,
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

func (ur *CachedUserRepository) FindByPhone(ctx context.Context, phone string) (domain.User, error) {
	user, err := ur.dao.FindByPhone(ctx, phone)
	return ur.entityToDomain(user), err
}

func (ur *CachedUserRepository) entityToDomain(ue dao.UserOfDB) domain.User {
	return domain.User{
		Id:       ue.Id,
		Email:    ue.Email.String,
		Password: ue.Password,
		Phone:    ue.Phone.String,
	}
}

func (ur *CachedUserRepository) domainToEntity(user domain.User) dao.UserOfDB {
	return dao.UserOfDB{
		Id: user.Id,
		Email: sql.NullString{
			String: user.Email,
			Valid:  user.Email != "",
		},
		Phone: sql.NullString{
			String: user.Phone,
			Valid:  user.Phone != "",
		},
		Password: user.Password,
	}
}
