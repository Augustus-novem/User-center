package main

import (
	"user-center/config"
	"user-center/internal/repository/dao"
	"user-center/internal/service/sms/localsms"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	db := initDB()
	Cmd := initRedis()
	sms := localsms.NewService()
	userSvc := initUserSvc(db, Cmd)
	codeSvc := initCodeSvc(Cmd, sms)
	server := initWebServer(userSvc, codeSvc)
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
