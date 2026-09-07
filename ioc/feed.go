package ioc

import (
	"user-center/internal/config"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/service"
	"user-center/pkg/logger"

	"github.com/google/wire"
	"github.com/redis/go-redis/v9"
)

var FeedSet = wire.NewSet(
	InitFeedInbox,
	wire.Bind(new(repository.FeedInbox), new(*cache.RedisFeedInbox)),
	InitFeedService,
)

func InitFeedInbox(cmd redis.Cmdable, cfg *config.AppConfig) *cache.RedisFeedInbox {
	return cache.NewRedisFeedInbox(cmd, cfg.Feed.InboxLimit())
}

func InitFeedService(
	inbox repository.FeedInbox,
	notes repository.NoteRepository,
	follows repository.FollowRepository,
	cfg *config.AppConfig,
	l logger.Logger,
) *service.FeedServiceImpl {
	return service.NewFeedServiceImpl(inbox, notes, follows, cfg.Feed.FanoutBatch(), l)
}
