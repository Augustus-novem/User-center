package jwt

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type Handler interface {
	ClearToken(ctx *gin.Context) error
	SetJWTToken(ctx *gin.Context, ssid string, uid int64) error
	SetLoginToken(ctx *gin.Context, uid int64) error
	ExtractAccessTokenString(ctx *gin.Context) string
	CheckSession(ctx *gin.Context, ssid string) error
	Refresh(ctx *gin.Context) error
	ParseAccessToken(tokenStr string) (*UserClaims, error)
}

type RefreshClaims struct {
	Id   int64
	Ssid string
	Jti  string
	jwt.RegisteredClaims
}

type UserClaims struct {
	Id        int64
	UserAgent string
	Ssid      string
	jwt.RegisteredClaims
}
