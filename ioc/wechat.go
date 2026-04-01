package ioc

import (
	"os"
	"user-center/internal/service/oauth2/wechat"
)

func InitWechatService() wechat.Service {
	appId, ok := os.LookupEnv("WECHAT_APP_ID")
	if !ok {
		panic("没有找到环境变量 WECHAT_APP_ID ")
	}
	appkey, ok := os.LookupEnv("WECHAT_APP_KEY")
	if !ok {
		panic("没有找到环境变量 WECHAT_APP_KEY")
	}
	return wechat.NewService(appId, appkey)
}
