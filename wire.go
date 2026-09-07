//go:build wireinject

package main

import (
	"user-center/internal/config"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/ioc"
	"user-center/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

func InitWebServer(cfg *config.AppConfig, dyn config.DynamicProvider, l logger.Logger) *gin.Engine {
	wire.Build(
		//基础部分
		ioc.InitDB, ioc.InitRedis, ioc.InitSmsService,
		ioc.InitWechatService, ioc.InitTX, ioc.InitEventPublisher,
		// DAO 部分
		dao.DAOSet,
		cache.CacheSet,
		repository.RepoSet,
		service.ServiceSet,
		ioc.FeedSet,

		ioc.InitJWTHandler,
		web.NewUserHandler,
		web.NewCheckInHandler,
		web.NewRankHandler,
		web.NewFollowHandler,
		web.NewNoteHandler,
		web.NewEngagementHandler,
		web.NewFeedHandler,
		ioc.GinMiddlewares,
		ioc.InitWebServer,
		ioc.InitOAuth2WechatHandler,
	)
	return nil
}
