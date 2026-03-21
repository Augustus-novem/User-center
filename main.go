package main

import (
	"strings"
	"time"
	"user-center/internal/repository"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/internal/web/middleware"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	db := initDB()
	server := initWebServer()
	initUser(db, server)
	server.Run(":8081")
}

func initDB() *gorm.DB {
	db, err := gorm.Open(mysql.Open("root:root@tcp(localhost:13316)/user_center"))
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

func initUser(db *gorm.DB, server *gin.Engine) {
	ud := dao.NewUserDao(db)
	ur := repository.NewUserRepo(ud)
	us := service.NewUserService(ur)
	uh := web.NewUserHandler(us)
	uh.Register(server)
}
