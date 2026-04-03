package jwt

import "github.com/google/wire"

var JWTSet = wire.NewSet(
	NewRedisHandlerWithConfig,
	wire.Bind(new(Handler), new(*RedisHandler)),
)
