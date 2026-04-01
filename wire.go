//go:build wireinject

package main

import (
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/ioc"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

func InitWebServer() *gin.Engine {
	wire.Build(
		//基础部分
		ioc.InitDB, ioc.InitRedis, ioc.InitSmsService, ioc.InitWechatService, ioc.InitTX,
		// DAO 部分
		dao.DAOSet,
		cache.CacheSet,
		repository.RepoSet,
		service.ServiceSet,

		web.NewUserHandler,
		web.NewOAuth2WechatHandler,
		ioc.GinMiddlwares,
		ioc.InitWebServer,
	)
	return nil
}
