package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"user-center/internal/domain"

	"github.com/redis/go-redis/v9"
)

type UserCache struct {
	cmd        redis.Cmdable
	expiration time.Duration
}

func NewUserCache(cmd redis.Cmdable) *UserCache {
	return &UserCache{
		cmd:        cmd,
		expiration: time.Minute * 15,
	}
}

func (cache *UserCache) Get(ctx context.Context,
	id int64) (domain.User, error) {
	key := cache.key(id)
	data, err := cache.cmd.Get(ctx, key).Result()
	if err != nil {
		return domain.User{}, err
	}
	var user domain.User
	err = json.Unmarshal([]byte(data), &user)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (cache *UserCache) Set(ctx context.Context, user domain.User) error {
	key := cache.key(user.Id)
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return cache.cmd.Set(ctx, key, data, cache.expiration).Err()
}

func (cache *UserCache) key(id int64) string {
	return fmt.Sprintf("user:info:%d", id)
}
