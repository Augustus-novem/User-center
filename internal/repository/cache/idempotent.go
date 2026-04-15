package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// IdempotentCache 幂等键缓存
type IdempotentCache interface {
	// SetIdempotentKey 设置幂等键，返回是否设置成功（true 表示首次设置）
	SetIdempotentKey(ctx context.Context, key string, ttl time.Duration) (bool, error)
	// CheckIdempotentKey 检查幂等键是否存在
	CheckIdempotentKey(ctx context.Context, key string) (bool, error)
	// DeleteIdempotentKey 删除幂等键
	DeleteIdempotentKey(ctx context.Context, key string) error
	// ExtendIdempotentKey 延长幂等键过期时间
	ExtendIdempotentKey(ctx context.Context, key string, ttl time.Duration) error
}

type RedisIdempotentCache struct {
	cmd    redis.Cmdable
	prefix string
}

func NewRedisIdempotentCache(cmd redis.Cmdable, prefix string) *RedisIdempotentCache {
	return &RedisIdempotentCache{
		cmd:    cmd,
		prefix: prefix,
	}
}

func (c *RedisIdempotentCache) SetIdempotentKey(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	fullKey := c.buildKey(key)
	// 使用 SET NX 保证原子性
	result, err := c.cmd.SetNX(ctx, fullKey, time.Now().Unix(), ttl).Result()
	return result, err
}

func (c *RedisIdempotentCache) CheckIdempotentKey(ctx context.Context, key string) (bool, error) {
	fullKey := c.buildKey(key)
	exists, err := c.cmd.Exists(ctx, fullKey).Result()
	return exists > 0, err
}

func (c *RedisIdempotentCache) DeleteIdempotentKey(ctx context.Context, key string) error {
	fullKey := c.buildKey(key)
	return c.cmd.Del(ctx, fullKey).Err()
}

func (c *RedisIdempotentCache) ExtendIdempotentKey(ctx context.Context, key string, ttl time.Duration) error {
	fullKey := c.buildKey(key)
	return c.cmd.Expire(ctx, fullKey, ttl).Err()
}

func (c *RedisIdempotentCache) buildKey(key string) string {
	return fmt.Sprintf("%s:%s", c.prefix, key)
}
