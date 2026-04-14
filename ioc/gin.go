package ioc

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"user-center/internal/config"
	"user-center/internal/web"
	jwt2 "user-center/internal/web/jwt"
	"user-center/internal/web/middleware"
	"user-center/pkg/ginx/middleware/accesslog"
	"user-center/pkg/ginx/middleware/ratelimit"
	pkglogger "user-center/pkg/logger"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func InitWebServer(cfg *config.AppConfig, funcs []gin.HandlerFunc,
	userHdl *web.UserHandler, oauth2Hdl *web.OAuth2WechatHandler,
	checkInHdl *web.CheckInHandler, rankHdl *web.RankHandler,
	ragHdl *web.RAGHandler) *gin.Engine {
	gin.SetMode(cfg.Server.Mode)
	server := gin.New()
	server.Use(gin.Recovery())
	server.Use(funcs...)
	userHdl.RegisterRoutes(server)
	oauth2Hdl.RegisterRoutes(server)
	checkInHdl.RegisterRoutes(server)
	rankHdl.RegisterRoutes(server)
	ragHdl.RegisterRoutes(server)
	return server
}

func GinMiddlewares(cfg *config.AppConfig, dyn config.DynamicProvider,
	cmd redis.Cmdable, hdl jwt2.Handler,
	logger pkglogger.Logger) []gin.HandlerFunc {
	res := make([]gin.HandlerFunc, 0)
	res = append(res, loggerMiddleware(logger))
	if cfg.RateLimit.Enabled {
		res = append(res,
			ratelimit.NewBuilder(cmd, cfg.RateLimit.Interval, cfg.RateLimit.Limit, logger).
				Prefix(cfg.RateLimit.Prefix).
				Build(),
		)
	}
	res = append(res, corsHandler(cfg.CORS),
		middleware.NewJWTLoginMiddlewareBuilder(hdl).Build(), featureGuardMiddleware(dyn))
	return res
}

func loggerMiddleware(l pkglogger.Logger) gin.HandlerFunc {
	return accesslog.NewMiddlewareBuilder(func(ctx context.Context, al accesslog.AccessLog) {
		l.Debug("GIN 收到请求", pkglogger.Field{
			Key:   "req",
			Value: al,
		})
	}).
		AllowBodyFor(http.MethodPost, "/user/login").
		AllowBodyFor(http.MethodPost, "/user/signup").
		AllowBodyFor(http.MethodPost, "/oauth2/wechat/authurl").
		SetBodyMasker(accessLogBodyMasker).
		Build()
}

func corsHandler(cfg config.CORSConfig) gin.HandlerFunc {
	return cors.New(
		cors.Config{
			AllowCredentials: cfg.AllowCredentials,
			AllowMethods:     cfg.AllowMethods,
			AllowOrigins:     cfg.AllowOrigins,
			AllowHeaders:     cfg.AllowHeaders,
			ExposeHeaders:    cfg.ExposeHeaders,
			MaxAge:           cfg.MaxAge,
		},
	)
}

func featureGuardMiddleware(dyn config.DynamicProvider) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		feature := dyn.Dynamic().Feature
		path := ctx.FullPath()

		switch path {
		case "/oauth2/wechat/authurl", "/oauth2/wechat/callback":
			if !feature.EnableWechatLogin {
				web.JSONBizError(ctx, "微信登录功能未开启")
				ctx.Abort()
				return
			}
		case "/user/login_sms", "/user/login_sms/code/send":
			if !feature.EnableSMSLogin {
				web.JSONBizError(ctx, "短信登录功能未开启")
				ctx.Abort()
				return
			}
		}

		ctx.Next()
	}
}

func accessLogBodyMasker(_ *gin.Context, body []byte) []byte {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return body
	}

	// 不是 JSON 就原样返回。
	// 如果你后面要支持表单脱敏，再单独加一套 form 处理。
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		return body
	}

	sensitiveKeys := map[string]struct{}{
		"password":      {},
		"passwd":        {},
		"pwd":           {},
		"token":         {},
		"access_token":  {},
		"refresh_token": {},
		"authorization": {},
		"code":          {},
		"phone":         {},
		"mobile":        {},
		"email":         {},
		"openid":        {},
		"unionid":       {},
	}

	maskJSONValue(data, sensitiveKeys)

	newBody, err := json.Marshal(data)
	if err != nil {
		return body
	}
	return newBody
}

func maskJSONValue(v any, sensitiveKeys map[string]struct{}) {
	switch val := v.(type) {
	case map[string]any:
		for k, child := range val {
			if _, ok := sensitiveKeys[strings.ToLower(k)]; ok {
				val[k] = maskLeafValue(child)
				continue
			}
			maskJSONValue(child, sensitiveKeys)
		}
	case []any:
		for _, item := range val {
			maskJSONValue(item, sensitiveKeys)
		}
	}
}

func maskLeafValue(v any) any {
	switch val := v.(type) {
	case string:
		return partialMask(val)
	default:
		return "***"
	}
}

func partialMask(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	n := len(runes)

	if n <= 2 {
		return strings.Repeat("*", n)
	}
	if n <= 6 {
		return string(runes[:1]) + strings.Repeat("*", n-2) + string(runes[n-1:])
	}
	return string(runes[:2]) + strings.Repeat("*", n-4) + string(runes[n-2:])
}
