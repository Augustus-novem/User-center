package main

import (
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/service/sms"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
)

func initUserSvc(db *gorm.DB, cmd redis.Cmdable) *service.UserService {
	ud := dao.NewUserDao(db)
	uc := cache.NewUserCache(cmd)
	ur := repository.NewUserRepo(ud, uc)
	us := service.NewUserService(ur)
	return us
}

func initCodeSvc(cmd redis.Cmdable, sms sms.Service) *service.CodeService {
	cc := cache.NewCodeCache(cmd)
	cr := repository.NewCodeRepository(cc)
	cs := service.NewCodeService(cr, sms)
	return cs
}
