package repository

import (
	"context"
	"user-center/internal/repository/cache"
)

var (
	ErrCodeVerifyTooManyTimes = cache.ErrCodeVerifyTooManyTimes
	ErrCodeSendTooMany        = cache.ErrCodeSendTooMany
)

type CodeRepository struct {
	CodeCache *cache.CodeCache
}

func NewCodeRepository(codeCache *cache.CodeCache) *CodeRepository {
	return &CodeRepository{
		CodeCache: codeCache,
	}
}

func (cr *CodeRepository) Verify(ctx context.Context,
	biz, phone, code string) (bool, error) {
	return cr.CodeCache.Verify(ctx, biz, phone, code)
}

func (cr *CodeRepository) Store(ctx context.Context, biz, phone, code string) error {
	return cr.CodeCache.Set(ctx, biz, phone, code)
}
