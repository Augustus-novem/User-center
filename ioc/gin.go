package ioc

import (
	"user-center/internal/config"
	"user-center/internal/web"
	jwt2 "user-center/internal/web/jwt"
	"user-center/internal/web/middleware"
	"user-center/pkg/ginx/middleware/ratelimit"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func InitWebServer(cfg *config.AppConfig, funcs []gin.HandlerFunc,
	userHdl *web.UserHandler, oauth2Hdl *web.OAuth2WechatHandler,
	checkInHdl *web.CheckInHandler, rankHdl *web.RankHandler) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)
	server := gin.Default()
	server.Use(gin.Logger(), gin.Recovery())
	server.Use(funcs...)
	userHdl.RegisterRoutes(server)
	oauth2Hdl.RegisterRoutes(server)
	checkInHdl.RegisterRoutes(server)
	rankHdl.RegisterRoutes(server)
	return server
}

func GinMiddlewares(cfg *config.AppConfig, dyn config.DynamicProvider, cmd redis.Cmdable, hdl jwt2.Handler) []gin.HandlerFunc {
	res := make([]gin.HandlerFunc, 0, 3)
	if cfg.RateLimit.Enabled {
		res = append(res,
			ratelimit.NewBuilder(cmd, cfg.RateLimit.Interval, cfg.RateLimit.Limit).
				Prefix(cfg.RateLimit.Prefix).
				Build(),
		)
	}
	res = append(res, corsHandler(cfg.CORS), middleware.NewJWTLoginMiddlewareBuilder(hdl).Build(), featureGuardMiddleware(dyn))
	return res
}

func corsHandler(cfg config.CORSConfig) gin.HandlerFunc {
	return cors.New(
		cors.Config{
			AllowCredentials: cfg.AllowCredentials,
			AllowMethods:     cfg.AllowMethods,
			AllowOrigins:     cfg.AllowOrigins,
			AllowHeaders:     cfg.AllowHeaders,
			ExposeHeaders:    cfg.ExposeHeaders,
			MaxAge:           cfg.MaxAge,
		},
	)
}

func featureGuardMiddleware(dyn config.DynamicProvider) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		feature := dyn.Dynamic().Feature
		path := ctx.FullPath()

		switch path {
		case "/oauth2/wechat/authurl", "/oauth2/wechat/callback":
			if !feature.EnableWechatLogin {
				web.JSONBizError(ctx, "微信登录功能未开启")
				ctx.Abort()
				return
			}
		case "/user/login_sms", "/user/login_sms/code/send":
			if !feature.EnableSMSLogin {
				web.JSONBizError(ctx, "短信登录功能未开启")
				ctx.Abort()
				return
			}
		}

		ctx.Next()
	}
}
