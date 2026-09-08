package cache

import (
	"context"
	"strings"
	"testing"
	"time"

	"user-center/internal/domain"

	"github.com/redis/go-redis/v9"
)

func TestRedisUserCacheDoesNotStorePasswordHash(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { _ = rdb.Close() })
	cache := NewRedisUserCache(rdb)
	user := domain.User{Id: time.Now().UnixNano(), Email: "cache@example.com", Password: "$2a$password-hash"}
	t.Cleanup(func() { _ = rdb.Del(context.Background(), cache.key(user.Id)).Err() })
	if err := cache.Set(ctx, user); err != nil {
		t.Fatal(err)
	}
	raw, err := rdb.Get(ctx, cache.key(user.Id)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "password") || strings.Contains(raw, user.Password) {
		t.Fatalf("cached user leaked password material: %s", raw)
	}
	got, err := cache.Get(ctx, user.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Password != "" || got.Email != user.Email {
		t.Fatalf("unexpected cached user: %+v", got)
	}
}
