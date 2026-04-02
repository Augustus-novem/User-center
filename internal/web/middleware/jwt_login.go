package middleware

import (
	"net/http"
	"time"
	jwt2 "user-center/internal/web/jwt"

	"github.com/ecodeclub/ekit/set"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type JWTLoginMiddlewareBuilder struct {
	publicPaths set.Set[string]
	jwt2.Handler
}

func NewJWTLoginMiddlewareBuilder(hdl jwt2.Handler) *JWTLoginMiddlewareBuilder {
	s := set.NewMapSet[string](2)
	s.Add("/user/signup")
	s.Add("/user/login_sms/code/send")
	s.Add("/user/login_sms")
	s.Add("/user/refresh_token")
	s.Add("/user/login")
	s.Add("/oauth2/wechat/authurl")
	s.Add("/oauth2/wechat/callback")
	return &JWTLoginMiddlewareBuilder{
		publicPaths: s,
		Handler:     hdl,
	}
}

func (j *JWTLoginMiddlewareBuilder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if j.publicPaths.Exist(ctx.Request.URL.Path) {
			return
		}
		tokenStr := j.ExtractAccessTokenString(ctx)
		uc := jwt2.UserClaims{}
		token, err := jwt.ParseWithClaims(tokenStr,
			&uc, func(token *jwt.Token) (interface{}, error) {
				return jwt2.AccessTokenKey, nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		)
		if err != nil || token == nil || !token.Valid {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		now := time.Now()
		expireAt, err := uc.GetExpirationTime()
		if err != nil || expireAt == nil || expireAt.Before(now) {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		if ctx.GetHeader("User-Agent") != uc.UserAgent {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		err = j.CheckSession(ctx, uc.Ssid)
		if err != nil {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		ctx.Set("user", uc)
	}
}
