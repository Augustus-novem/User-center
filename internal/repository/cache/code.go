package cache

import (
	"context"
	_ "embed"
	"errors"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	//go:embed lua/verify_code.lua
	luaVerifyCode string
	//go:embed lua/set_code.lua
	luaSetCode                string
	ErrCodeVerifyTooManyTimes = errors.New("验证次数太多")
	ErrCodeSendTooMany        = errors.New("发送验证码太频繁")
	ErrUnknownForCode         = errors.New("发送验证码遇到未知错误")
)

type CodeCache struct {
	cmd redis.Cmdable
}

func NewCodeCache(cmd redis.Cmdable) *CodeCache {
	return &CodeCache{
		cmd: cmd,
	}
}

func (cc *CodeCache) Set(ctx context.Context,
	biz, phone, code string) error {
	result, err := cc.cmd.Eval(ctx, luaSetCode,
		[]string{cc.key(biz, phone)}, code).Int()
	if err != nil {
		return err
	}
	switch result {
	case 0:
		return nil
	case -1:
		return ErrCodeSendTooMany
	default:
		return ErrUnknownForCode
	}
}

func (cc *CodeCache) Verify(ctx context.Context,
	biz, phone, inputCode string) (bool, error) {
	result, err := cc.cmd.Eval(ctx, luaVerifyCode, []string{cc.key(biz, phone)}, inputCode).Int()
	if err != nil {
		return false, err
	}
	switch result {
	case 0:
		return true, nil
	case -1:
		return false, ErrCodeVerifyTooManyTimes
	default:
		return false, nil
	}
}

func (cc *CodeCache) key(biz, phone string) string {
	return fmt.Sprintf("phone_code:%s:%s", biz, phone)
}
