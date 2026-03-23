package middleware

import (
	"net/http"
	"strings"
	"time"
	"user-center/internal/web"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type JWTLoginMiddlewareBuilder struct {
}

func (j *JWTLoginMiddlewareBuilder) Build() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		path := ctx.Request.URL.Path
		if path == "/user/login" || path == "/user/signup" {
			return
		}
		authCode := ctx.Request.Header.Get("Authorization")
		if authCode == "" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		authSegments := strings.SplitN(authCode, " ", 2)
		if len(authSegments) != 2 || authSegments[0] != "Bearer" {
			ctx.AbortWithStatus(http.StatusUnauthorized)
			return
		}
		tokenStr := authSegments[1]
		uc := web.UserClaims{}
		token, err := jwt.ParseWithClaims(tokenStr,
			&uc, func(token *jwt.Token) (interface{}, error) {
				return web.JWTKey, nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		)
		if err != nil || !token.Valid {
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
		if expireAt.Sub(now) < time.Minute*20 {
			uc.ExpiresAt = jwt.NewNumericDate(now.Add(time.Minute))
			newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, uc)
			newTokenStr, err := newToken.SignedString(web.JWTKey)
			if err == nil {
				ctx.Header("x-jwt-token", newTokenStr)
			}
		}
		ctx.Set("user", uc)
	}
}
