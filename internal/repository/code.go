package repository

import (
	"context"
	"user-center/internal/repository/cache"
)

var (
	ErrCodeVerifyTooManyTimes = cache.ErrCodeVerifyTooManyTimes
	ErrCodeSendTooMany        = cache.ErrCodeSendTooMany
)

type CodeRepository interface {
	Store(ctx context.Context, biz, phone, code string) error
	Verify(ctx context.Context, biz, phone, inputCode string) (bool, error)
}

type CachedCodeRepository struct {
	CodeCache cache.CodeCache
}

func NewCachedCodeRepository(codeCache cache.CodeCache) *CachedCodeRepository {
	return &CachedCodeRepository{
		CodeCache: codeCache,
	}
}

func (cr *CachedCodeRepository) Verify(ctx context.Context,
	biz, phone, code string) (bool, error) {
	return cr.CodeCache.Verify(ctx, biz, phone, code)
}

func (cr *CachedCodeRepository) Store(ctx context.Context, biz, phone, code string) error {
	return cr.CodeCache.Set(ctx, biz, phone, code)
}
