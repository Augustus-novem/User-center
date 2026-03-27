package main

import (
	"strings"
	"time"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/internal/web/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func initWebServer(userSvc *service.UserService, codeSvc *service.CodeService) *gin.Engine {
	server := gin.Default()
	server.Use(cors.New(
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
	))
	usingJWT(server)

	userHdl := web.NewUserHandler(userSvc, codeSvc)
	userHdl.Register(server)

	return server
}

func usingJWT(server *gin.Engine) {
	builder := middleware.NewJWTLoginMiddlewareBuilder()
	server.Use(builder.Build())
}
