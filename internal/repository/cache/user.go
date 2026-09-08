package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"user-center/internal/domain"

	"github.com/redis/go-redis/v9"
)

type UserCache interface {
	Get(ctx context.Context, id int64) (domain.User, error)
	Set(ctx context.Context, user domain.User) error
	Delete(ctx context.Context, id int64) error
}

type RedisUserCache struct {
	cmd        redis.Cmdable
	expiration time.Duration
}

type userCacheDTO struct {
	ID       int64     `json:"id"`
	Email    string    `json:"email"`
	Phone    string    `json:"phone"`
	NickName string    `json:"nickname"`
	AboutMe  string    `json:"about_me"`
	Birthday time.Time `json:"birthday"`
	Ctime    int64     `json:"ctime"`
}

func NewRedisUserCache(cmd redis.Cmdable) *RedisUserCache {
	return &RedisUserCache{
		cmd:        cmd,
		expiration: time.Minute * 15,
	}
}

func (cache *RedisUserCache) Get(ctx context.Context,
	id int64) (domain.User, error) {
	key := cache.key(id)
	data, err := cache.cmd.Get(ctx, key).Result()
	if err != nil {
		return domain.User{}, err
	}
	var dto userCacheDTO
	err = json.Unmarshal([]byte(data), &dto)
	if err != nil {
		return domain.User{}, err
	}
	return domain.User{
		Id: dto.ID, Email: dto.Email, Phone: dto.Phone,
		NickName: dto.NickName, AboutMe: dto.AboutMe,
		Birthday: dto.Birthday, Ctime: dto.Ctime,
	}, nil
}

func (cache *RedisUserCache) Set(ctx context.Context, user domain.User) error {
	key := cache.key(user.Id)
	data, err := json.Marshal(userCacheDTO{
		ID: user.Id, Email: user.Email, Phone: user.Phone,
		NickName: user.NickName, AboutMe: user.AboutMe,
		Birthday: user.Birthday, Ctime: user.Ctime,
	})
	if err != nil {
		return err
	}
	return cache.cmd.Set(ctx, key, data, cache.expiration).Err()
}

func (cache *RedisUserCache) Delete(ctx context.Context, id int64) error {
	return cache.cmd.Del(ctx, cache.key(id)).Err()
}

func (cache *RedisUserCache) key(id int64) string {
	return fmt.Sprintf("user:info:%d", id)
}
