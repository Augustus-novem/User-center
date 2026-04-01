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
	FindOrCreateByWechat(ctx context.Context, info domain.SocialAccount) (domain.User, error)
	SignUp(ctx context.Context, user domain.User) error
	Profile(ctx context.Context, id int64) (domain.User, error)
}

type UserServiceImpl struct {
	UserRepo   repository.UserRepository
	SocialRepo repository.SocialAccountRepository
	Tx         repository.Transaction
}

func NewUserServiceImpl(userRepo repository.UserRepository,
	socialRepo repository.SocialAccountRepository,
	tx repository.Transaction) *UserServiceImpl {
	return &UserServiceImpl{
		UserRepo:   userRepo,
		SocialRepo: socialRepo,
		Tx:         tx,
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

func (us *UserServiceImpl) FindOrCreateByWechat(ctx context.Context, info domain.SocialAccount) (domain.User, error) {
	acc, err := us.SocialRepo.FindByProviderAndOpenID(ctx, domain.OAuthProviderWechat, info.OpenId)
	if err == nil {
		return us.UserRepo.FindByID(ctx, acc.UserId)
	}
	if !errors.Is(err, repository.ErrSocialAccountNotFound) {
		return domain.User{}, err
	}
	newUser := domain.User{}
	newSocialAcc := domain.SocialAccount{
		Provider: domain.OAuthProviderWechat,
		OpenId:   info.OpenId,
		UnionId:  info.UnionId,
	}
	err = us.Tx.InTx(ctx, func(txCtx context.Context) error {
		// 1. 创建本地用户，并拿到自增 ID
		user, err := us.UserRepo.CreateAndReturn(txCtx, newUser)
		if err != nil {
			return err // 返回错误，自动触发整个事务回滚
		}
		newUser = user
		// 2. 把拿到的 userID 塞给第三方账号绑定信息
		newSocialAcc.UserId = user.Id
		// 3. 创建绑定关系
		err = us.SocialRepo.Create(txCtx, newSocialAcc)
		if err != nil {
			return err // 返回错误，自动触发回滚，刚创建的 User 也被撤销
		}
		return nil // 一切顺利，自动提交！
	})
	if err != nil {
		return domain.User{}, err
	}
	return newUser, nil
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
