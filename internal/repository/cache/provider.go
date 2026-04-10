package cache

import "github.com/google/wire"

var CacheSet = wire.NewSet(
	NewRedisCodeCache,
	wire.Bind(new(CodeCache), new(*RedisCodeCache)),
	NewRedisUserCache,
	wire.Bind(new(UserCache), new(*RedisUserCache)),
	NewRedisRankCache,
	wire.Bind(new(RankCache), new(*RedisRankCache)),
	NewRedisSignInCache,
	wire.Bind(new(SignInCache), new(*RedisSignInCache)),
)
