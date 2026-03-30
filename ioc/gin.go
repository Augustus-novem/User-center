package ioc

import (
	"strings"
	"time"
	"user-center/internal/web"
	"user-center/internal/web/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func InitWebServer(funcs []gin.HandlerFunc,
	userHdl *web.UserHandler) *gin.Engine {
	server := gin.Default()
	server.Use(funcs...)
	userHdl.RegisterRoutes(server)
	return server
}

func GinMiddlwares(cmd redis.Cmdable) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		corsHandler(),
		middleware.NewJWTLoginMiddlewareBuilder().Build(),
	}
}

func corsHandler() gin.HandlerFunc {
	return cors.New(
		cors.Config{
			AllowCredentials: true,
			AllowHeaders:     []string{"Content-Type", "Authorization"},
			ExposeHeaders:    []string{"x-jwt-token"},
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
