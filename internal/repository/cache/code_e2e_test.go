//go:build e2e

package cache

import (
	"context"
	"testing"
	"time"
	"user-center/internal/config"

	"github.com/redis/go-redis/v9"
)

func newTestRedis(t *testing.T) *redis.Client {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Config.Redis.Addr,
		Password: config.Config.Redis.Password,
		DB:       config.Config.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	return rdb
}

func TestRedisCodeCache_Set_e2e(t *testing.T) {
	rdb := newTestRedis(t)
	c := NewRedisCodeCache(rdb)

	testCases := []struct {
		name    string
		before  func(t *testing.T)
		after   func(t *testing.T)
		ctx     context.Context
		biz     string
		phone   string
		code    string
		wantErr error
	}{
		{
			name:   "验证码存储成功",
			before: func(t *testing.T) {},
			after: func(t *testing.T) {
				ctx := context.Background()
				key := c.key("login", "15212345678")
				val, err := rdb.Get(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if val != "123456" {
					t.Fatalf("want code 123456, got %s", val)
				}
				ttl, err := rdb.TTL(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if ttl <= 9*time.Minute {
					t.Fatalf("want ttl > 9m, got %v", ttl)
				}
				if err = rdb.Del(ctx, key, key+":cnt").Err(); err != nil {
					t.Fatal(err)
				}
			},
			ctx: context.Background(), biz: "login", phone: "15212345678", code: "123456",
		},
		{
			name: "发送太频繁",
			before: func(t *testing.T) {
				ctx := context.Background()
				key := c.key("login", "15212345679")
				if err := rdb.Set(ctx, key, "123456", 9*time.Minute+30*time.Second).Err(); err != nil {
					t.Fatal(err)
				}
			},
			after: func(t *testing.T) {
				ctx := context.Background()
				key := c.key("login", "15212345679")
				val, err := rdb.Get(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if val != "123456" {
					t.Fatalf("want preserved code 123456, got %s", val)
				}
				if err = rdb.Del(ctx, key, key+":cnt").Err(); err != nil {
					t.Fatal(err)
				}
			},
			ctx: context.Background(), biz: "login", phone: "15212345679", code: "234567", wantErr: ErrCodeSendTooMany,
		},
		{
			name: "未知错误",
			before: func(t *testing.T) {
				ctx := context.Background()
				key := c.key("login", "15212345670")
				if err := rdb.Set(ctx, key, "123456", 0).Err(); err != nil {
					t.Fatal(err)
				}
			},
			after: func(t *testing.T) {
				ctx := context.Background()
				key := c.key("login", "15212345670")
				val, err := rdb.Get(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if val != "123456" {
					t.Fatalf("want preserved code 123456, got %s", val)
				}
				if err = rdb.Del(ctx, key, key+":cnt").Err(); err != nil {
					t.Fatal(err)
				}
			},
			ctx: context.Background(), biz: "login", phone: "15212345670", code: "234567", wantErr: ErrUnknownForCode,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.before(t)
			err := c.Set(tc.ctx, tc.biz, tc.phone, tc.code)
			if err != tc.wantErr {
				t.Fatalf("want err %v, got %v", tc.wantErr, err)
			}
			tc.after(t)
		})
	}
}
