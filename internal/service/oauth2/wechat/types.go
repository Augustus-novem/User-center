package wechat

import (
	"context"
	"user-center/internal/domain"
)

type Service interface {
	AuthURL(ctx context.Context, state string) (string, error)
	VerifyCode(ctx context.Context, code string) (domain.SocialAccount, error)
}
