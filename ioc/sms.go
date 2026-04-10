package ioc

import (
	"user-center/internal/service/sms"
	"user-center/internal/service/sms/localsms"
	"user-center/pkg/logger"
)

func InitSmsService(l logger.Logger) sms.Service {
	return InitSmsMemoryService(l)
}
func InitSmsMemoryService(l logger.Logger) sms.Service {
	return localsms.NewService(l)
}
