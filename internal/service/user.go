package service

import (
	"context"
	"errors"
	"user-center/internal/domain"
	"user-center/internal/repository"

	"golang.org/x/crypto/bcrypt"
)

var ErrUserDuplicate = repository.ErrUserDuplicate
var ErrInvalidUserOrPassword = errors.New("用户名或密码不正确")

type UserService interface {
	Login(ctx context.Context, email, password string) (domain.User, error)
	FindOrCreate(ctx context.Context, phone string) (domain.User, error)
	SignUp(ctx context.Context, user domain.User) error
	Profile(ctx context.Context, id int64) (domain.User, error)
}

type UserServiceImpl struct {
	UserRepo repository.UserRepository
}

func NewUserServiceImpl(userRepo repository.UserRepository) *UserServiceImpl {
	return &UserServiceImpl{
		UserRepo: userRepo,
	}
}

func (us *UserServiceImpl) SignUp(c context.Context, user domain.User) error {
	//加密
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	return us.UserRepo.Create(c, user)
}

func (us *UserServiceImpl) FindOrCreate(ctx context.Context, phone string) (domain.User, error) {
	user, err := us.UserRepo.FindByPhone(ctx, phone)
	if !errors.Is(err, repository.ErrUserNotFound) {
		return user, err
	}
	err = us.UserRepo.Create(ctx, domain.User{
		Phone: phone,
	})
	if err != nil && !errors.Is(err, repository.ErrUserDuplicate) {
		return domain.User{}, err
	}
	return us.UserRepo.FindByPhone(ctx, phone)
}

func (us *UserServiceImpl) Login(c context.Context,
	email, password string) (domain.User, error) {
	user, err := us.UserRepo.FindByEmail(c, email)
	if errors.Is(err, repository.ErrUserNotFound) {
		return domain.User{}, ErrInvalidUserOrPassword
	}
	if err != nil {
		return domain.User{}, err
	}
	err = bcrypt.CompareHashAndPassword([]byte(user.Password),
		[]byte(password))
	if err != nil {
		return domain.User{}, ErrInvalidUserOrPassword
	}
	return user, nil
}

func (us *UserServiceImpl) Profile(ctx context.Context, id int64) (domain.User, error) {
	return us.UserRepo.FindByID(ctx, id)
}
