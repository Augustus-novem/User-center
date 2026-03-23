package main

import (
	"strings"
	"time"
	"user-center/config"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/internal/web/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	db := initDB()
	Cmd := initRedis()
	server := initWebServer()
	initUser(server, db, Cmd)
	server.Run(":8081")
}

func initRedis() redis.Cmdable {
	redisCfg := config.Config.Redis
	Cmd := redis.NewClient(&redis.Options{
		Addr:     redisCfg.Addr,
		Password: redisCfg.Password,
		DB:       redisCfg.DB,
	})
	return Cmd
}

func initDB() *gorm.DB {
	db, err := gorm.Open(mysql.Open(config.Config.DB.DSN))
	if err != nil {
		panic("failed to connect database")
	}
	err = dao.InitTables(db)
	if err != nil {
		panic(err)
	}
	return db
}

func initWebServer() *gin.Engine {
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
	return server
}

func usingJWT(server *gin.Engine) {
	builder := &middleware.JWTLoginMiddlewareBuilder{}
	server.Use(builder.Build())
}

func initUser(server *gin.Engine, db *gorm.DB, cmd redis.Cmdable) {
	ud := dao.NewUserDao(db)
	uc := cache.NewUserCache(cmd)
	ur := repository.NewUserRepo(ud, uc)
	us := service.NewUserService(ur)
	uh := web.NewUserHandler(us)
	uh.Register(server)
}
