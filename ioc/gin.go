package ioc

import (
	"strings"
	"time"
	"user-center/internal/web"
	jwt2 "user-center/internal/web/jwt"
	"user-center/internal/web/middleware"
	"user-center/pkg/ginx/middleware/ratelimit"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func InitWebServer(funcs []gin.HandlerFunc,
	userHdl *web.UserHandler, oauth2Hdl *web.OAuth2WechatHandler) *gin.Engine {
	server := gin.Default()
	server.Use(funcs...)
	userHdl.RegisterRoutes(server)
	oauth2Hdl.RegisterRoutes(server)
	return server
}

func GinMiddlwares(cmd redis.Cmdable, hdl jwt2.Handler) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		ratelimit.NewBuilder(cmd, time.Minute, 100).Build(),
		corsHandler(),
		middleware.NewJWTLoginMiddlewareBuilder(hdl).Build(),
	}
}

func corsHandler() gin.HandlerFunc {
	return cors.New(
		cors.Config{
			AllowCredentials: true,
			AllowHeaders: []string{
				"Content-Type",
				"Authorization",
				"X-Refresh-Token",
			},
			ExposeHeaders: []string{
				"x-jwt-token",
				"x-refresh-token",
			},
			AllowOriginFunc: func(origin string) bool {
				if strings.HasPrefix(origin, "http://localhost") {
					return true
				}
				return strings.Contains(origin, "you_company.com")
			},
			MaxAge: 12 * time.Hour,
		},
	)
}
