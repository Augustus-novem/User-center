package ioc

import appconfig "user-center/internal/config"

func InitConfig() (*appconfig.Manager, error) {
	return appconfig.NewManagerFromFlags()
}
