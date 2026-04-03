package ioc

import (
	"user-center/internal/config"
	"user-center/internal/service"
	"user-center/internal/service/oauth2/wechat"
	"user-center/internal/web"
	jwt2 "user-center/internal/web/jwt"
)

func InitWechatService(cfg *config.AppConfig) wechat.Service {
	return wechat.NewService(cfg.Wechat.AppID, cfg.Wechat.AppKey, cfg.Wechat.RedirectURL)
}

func InitOAuth2WechatHandler(cfg *config.AppConfig,
	wechatSvc wechat.Service,
	userSvc service.UserService,
	jwtHdl jwt2.Handler) *web.OAuth2WechatHandler {
	return web.NewOAuth2WechatHandlerWithConfig(wechatSvc, userSvc, jwtHdl, cfg.Wechat)
}
