package jwt

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	//go:embed lua/clear_token.lua
	luaClearTokenCode string
	//go:embed lua/compare_and_set.lua
	luaRotateRefreshCode      string
	RefreshTokenKey           = []byte("PYB340i9IPNMJYVxT4ZPTWxa882JTGScFvhG4IGib6h")
	AccessTokenKey            = []byte("48lJdJICc98rxdTD47otl9y3w78PGePJn9QW0vlgCXD")
	ErrJWTTokenInvalid        = errors.New("jwt token invalid")
	ErrJWTTokenSessionExpired = errors.New("jwt token session expired")
)

type RedisHandler struct {
	cmd         redis.Cmdable
	atTTL       time.Duration
	rtTTL       time.Duration
	idleTTL     time.Duration
	absoluteTTL time.Duration
}

func NewRedisHandler(cmd redis.Cmdable) *RedisHandler {
	return &RedisHandler{
		cmd:         cmd,
		atTTL:       15 * time.Minute,
		rtTTL:       7 * 24 * time.Hour,
		idleTTL:     7 * 24 * time.Hour,
		absoluteTTL: 30 * 24 * time.Hour,
	}
}

func (h *RedisHandler) ClearToken(ctx *gin.Context) error {
	ctx.Header("x-jwt-token", "")
	ctx.Header("x-refresh-token", "")
	uc, ok := ctx.Get("user")
	if !ok {
		return ErrJWTTokenInvalid
	}
	userClaims, ok := uc.(UserClaims)
	if !ok {
		return ErrJWTTokenInvalid
	}
	_, err := h.cmd.Eval(
		ctx.Request.Context(),
		luaClearTokenCode,
		[]string{
			h.refreshKey(userClaims.Ssid),
			h.key(userClaims.Ssid),
		},
		int(h.atTTL/time.Second),
	).Int()
	return err
}

func (h *RedisHandler) SetRefreshToken(ctx *gin.Context, ssid string, uid int64, jti string) error {
	rc := RefreshClaims{
		Id:   uid,
		Ssid: ssid,
		Jti:  jti,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(h.rtTTL)),
		},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, rc)
	tokenStr, err := refreshToken.SignedString(RefreshTokenKey)
	if err != nil {
		return err
	}
	ctx.Header("x-refresh-token", tokenStr)
	return nil
}

func (h *RedisHandler) storeRefreshJTI(ctx *gin.Context, ssid string, jti string) error {
	return h.cmd.Set(ctx.Request.Context(), h.refreshKey(ssid), jti, h.rtTTL).Err()
}

func (h *RedisHandler) SetJWTToken(ctx *gin.Context, ssid string, uid int64) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		Id:        uid,
		Ssid:      ssid,
		UserAgent: ctx.GetHeader("User-Agent"),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(h.atTTL)),
		},
	})
	tokenStr, err := token.SignedString(AccessTokenKey)
	if err != nil {
		return err
	}
	ctx.Header("x-jwt-token", tokenStr)
	return nil
}

func (h *RedisHandler) SetLoginToken(ctx *gin.Context, uid int64) error {
	ssid := uuid.New().String()
	jti := uuid.New().String()

	if err := h.storeRefreshJTI(ctx, ssid, jti); err != nil {
		return err
	}
	if err := h.SetJWTToken(ctx, ssid, uid); err != nil {
		return err
	}
	return h.SetRefreshToken(ctx, ssid, uid, jti)
}

func (h *RedisHandler) CheckSession(ctx *gin.Context, ssid string) error {
	logout, err := h.cmd.Exists(ctx.Request.Context(), h.key(ssid)).Result()
	if err != nil {
		return err
	}
	if logout > 0 {
		return errors.New("用户已退出登录")
	}
	return nil
}

func (h *RedisHandler) key(ssid string) string {
	return fmt.Sprintf("user:ssid:%s", ssid)
}

func (h *RedisHandler) refreshKey(ssid string) string {
	return fmt.Sprintf("user:refresh:ssid:%s", ssid)
}

func (h *RedisHandler) ExtractAccessTokenString(ctx *gin.Context) string {
	authCode := ctx.GetHeader("Authorization")
	if authCode == "" {
		return ""
	}
	authSegments := strings.SplitN(authCode, " ", 2)
	if len(authSegments) != 2 || authSegments[0] != "Bearer" {
		return ""
	}
	return authSegments[1]
}

func (h *RedisHandler) Refresh(ctx *gin.Context) error {
	tokenStr := ctx.GetHeader("x-refresh-token")
	if tokenStr == "" {
		return ErrJWTTokenInvalid
	}
	var rc RefreshClaims
	token, err := jwt.ParseWithClaims(tokenStr, &rc, func(token *jwt.Token) (interface{}, error) {
		return RefreshTokenKey, nil
	})
	if err != nil || token == nil || !token.Valid {
		return ErrJWTTokenInvalid
	}
	newJTI := uuid.New().String()
	res, err := h.cmd.Eval(ctx.Request.Context(),
		luaRotateRefreshCode,
		[]string{h.refreshKey(rc.Ssid)},
		rc.Jti, newJTI, int(h.rtTTL/time.Second)).Int()
	if err != nil {
		return err
	}
	if res != 1 {
		return ErrJWTTokenInvalid
	}
	if err = h.SetJWTToken(ctx, rc.Ssid, rc.Id); err != nil {
		return err
	}
	return h.SetRefreshToken(ctx, rc.Ssid, rc.Id, newJTI)
}
