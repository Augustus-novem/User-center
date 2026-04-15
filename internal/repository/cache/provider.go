package cache

import (
	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

var CacheSet = wire.NewSet(
	NewRedisCodeCache,
	wire.Bind(new(CodeCache), new(*RedisCodeCache)),
	NewRedisUserCache,
	wire.Bind(new(UserCache), new(*RedisUserCache)),
	NewRedisRankCache,
	wire.Bind(new(RankCache), new(*RedisRankCache)),
	NewRedisSignInCache,
	wire.Bind(new(SignInCache), new(*RedisSignInCache)),
	NewRankConsistencyCache,
	NewRedisIdempotentCacheWithPrefix,
	wire.Bind(new(IdempotentCache), new(*RedisIdempotentCache)),
)

func NewRedisIdempotentCacheWithPrefix(cmd redis.Cmdable) *RedisIdempotentCache {
	return NewRedisIdempotentCache(cmd, "idempotent")
}
