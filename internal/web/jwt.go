package web

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type UserClaims struct {
	Id        int64
	UserAgent string
	jwt.RegisteredClaims
}

type StateClaims struct {
	State string
	jwt.RegisteredClaims
}

type jwtHandler struct{}

var JWTKey = []byte("moyn8y9abnd7q4zkq2m73yw8tu9j5ixm")

func (u *jwtHandler) setJWTToken(ctx *gin.Context, id int64) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		Id:        id,
		UserAgent: ctx.GetHeader("User-Agent"),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 30)),
		},
	})
	tokenStr, err := token.SignedString(JWTKey)
	if err != nil {
		return err
	}
	ctx.Header("x-jwt-token", tokenStr)
	return nil
}
