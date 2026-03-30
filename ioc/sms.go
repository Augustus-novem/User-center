package ioc

import (
	"user-center/internal/service/sms"
	"user-center/internal/service/sms/localsms"
)

func InitSmsService() sms.Service {
	return InitSmsMemoryService()
}
func InitSmsMemoryService() sms.Service {
	return localsms.NewService()
}
