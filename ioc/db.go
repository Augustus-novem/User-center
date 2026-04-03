package ioc

import (
	"user-center/internal/config"
	"user-center/internal/repository/dao"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func InitDB(cfg *config.AppConfig) *gorm.DB {
	db, err := gorm.Open(mysql.Open(cfg.DB.DSN))
	if err != nil {
		panic("failed to connect database")
	}
	err = dao.InitTables(db)
	if err != nil {
		panic(err)
	}
	return db
}
