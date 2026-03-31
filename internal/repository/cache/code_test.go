package cache

import (
	"context"
	"testing"

	"github.com/redis/go-redis/v9"
)

type fakeCmdable struct {
	redis.Cmdable
	evalFn func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd
}

func (f *fakeCmdable) Eval(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
	return f.evalFn(ctx, script, keys, args...)
}

func TestRedisCodeCache_Set(t *testing.T) {
	testCases := []struct {
		name    string
		cmd     redis.Cmdable
		ctx     context.Context
		biz     string
		phone   string
		code    string
		wantErr error
	}{
		{
			name: "设置成功",
			cmd: &fakeCmdable{evalFn: func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
				return redis.NewCmdResult(int64(0), nil)
			}},
			ctx: context.Background(),
			biz: "login", phone: "15212345678", code: "123456",
		},
		{
			name: "发送太频繁",
			cmd: &fakeCmdable{evalFn: func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
				return redis.NewCmdResult(int64(-1), nil)
			}},
			ctx: context.Background(),
			biz: "login", phone: "15212345678", code: "123456",
			wantErr: ErrCodeSendTooMany,
		},
		{
			name: "系统错误",
			cmd: &fakeCmdable{evalFn: func(ctx context.Context, script string, keys []string, args ...any) *redis.Cmd {
				return redis.NewCmdResult(int64(-2), nil)
			}},
			ctx: context.Background(),
			biz: "login", phone: "15212345678", code: "123456",
			wantErr: ErrUnknownForCode,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			c := NewRedisCodeCache(tc.cmd)
			err := c.Set(tc.ctx, tc.biz, tc.phone, tc.code)
			if err != tc.wantErr {
				t.Fatalf("want err %v, got %v", tc.wantErr, err)
			}
		})
	}
}
