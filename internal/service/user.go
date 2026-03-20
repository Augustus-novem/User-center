package service

import (
	"context"
	"golang.org/x/crypto/bcrypt"
	"user-center/internal/domain"
	"user-center/internal/repository"
)

var ErrUserDuplicateEmail = repository.ErrUserDuplicateEmail

type UserService struct {
	UserRepo *repository.UserRepo
}

func NewUserService(userRepo *repository.UserRepo) *UserService {
	return &UserService{
		UserRepo: userRepo,
	}
}

func (u *UserService) SignUp(c context.Context, user domain.User) error {
	//加密
	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user.Password = string(hash)
	return u.UserRepo.Create(c, user)
}
