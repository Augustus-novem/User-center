package ioc

import (
	"time"
	"user-center/internal/config"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/service"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

var HotRankSet = wire.NewSet(
	InitHotRankCache,
	wire.Bind(new(cache.HotRankCache), new(*cache.RedisHotRankCache)),
	repository.NewHotRankRepositoryImpl,
	wire.Bind(new(repository.HotRankRepository), new(*repository.HotRankRepositoryImpl)),
	InitHotRankService,
	wire.Bind(new(service.HotRankService), new(*service.HotRankServiceImpl)),
)

func InitHotRankCache(cmd redis.Cmdable, cfg *config.AppConfig) *cache.RedisHotRankCache {
	return cache.NewRedisHotRankCache(cmd,
		time.Duration(cfg.HotRank.WindowMinutes)*time.Minute,
		cfg.HotRank.SnapshotTTL,
		cfg.HotRank.EventDedupTTL,
	)
}

func InitHotRankService(repo repository.HotRankRepository, cfg *config.AppConfig) *service.HotRankServiceImpl {
	return service.NewHotRankServiceImpl(repo, service.HotRankWeights{
		Publish: cfg.HotRank.PublishWeight,
		Like:    cfg.HotRank.LikeWeight,
		Comment: cfg.HotRank.CommentWeight,
	})
}
