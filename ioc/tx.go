package ioc

import (
	"user-center/internal/repository"
	"user-center/internal/repository/dao"

	"gorm.io/gorm"
)

func InitTX(db *gorm.DB) repository.Transaction {
	return dao.NewGORMTransaction(db)
}
