package ioc

import (
	"user-center/internal/config"
	jwt2 "user-center/internal/web/jwt"

	"github.com/redis/go-redis/v9"
)

func InitJWTHandler(cfg *config.AppConfig, cmd redis.Cmdable) jwt2.Handler {
	return jwt2.NewRedisHandlerWithConfig(cmd, cfg.JWT)
}
