package ioc

import (
	"user-center/config"

	"github.com/redis/go-redis/v9"
)

func InitRedis() redis.Cmdable {
	redisCfg := config.Config.Redis
	Cmd := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	return Cmd
}
