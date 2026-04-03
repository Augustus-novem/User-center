package ioc

import (
	"user-center/internal/config"

	"github.com/redis/go-redis/v9"
)

func InitRedis(cfg *config.AppConfig) redis.Cmdable {
	redisCfg := cfg.Redis
	Cmd := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	return Cmd
}
