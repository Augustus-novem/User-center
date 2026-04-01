package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var (
	ErrSocialAccountNotFound = dao.ErrAccountNotFound
)

type SocialAccountRepository interface {
	Create(ctx context.Context, sa domain.SocialAccount) error
	FindByProviderAndOpenID(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error)
}

type SocialAccountRepositoryImpl struct {
	dao dao.SocialAccountDAO
}

func NewSocialAccountRepositoryImpl(dao dao.SocialAccountDAO) *SocialAccountRepositoryImpl {
	return &SocialAccountRepositoryImpl{dao: dao}
}

func (s *SocialAccountRepositoryImpl) Create(ctx context.Context, sa domain.SocialAccount) error {
	return s.dao.Insert(ctx, dao.SocialAccountOfDB{
		UserId:   sa.UserId,
		Provider: string(sa.Provider),
		OpenId:   sa.OpenId,
		UnionId:  sa.UnionId,
	})
}

func (s *SocialAccountRepositoryImpl) FindByProviderAndOpenID(ctx context.Context, provider domain.OAuthProvider, openID string) (domain.SocialAccount, error) {
	sa, err := s.dao.FindByProviderAndOpenID(ctx, string(provider), openID)
	if err != nil {
		return domain.SocialAccount{}, err
	}
	return domain.SocialAccount{
		Id:       sa.Id,
		UserId:   sa.UserId,
		Provider: domain.OAuthProvider(sa.Provider),
		OpenId:   sa.OpenId,
		UnionId:  sa.UnionId,
		Ctime:    sa.Ctime,
	}, nil
}
