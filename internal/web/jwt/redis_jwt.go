package jwt

import (
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	appconfig "user-center/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var (
	//go:embed lua/clear_token.lua
	luaClearTokenCode string
	//go:embed lua/compare_and_set.lua
	luaRotateRefreshCode string
	//go:embed lua/create_session.lua
	luaCreateSessionCode string
	//go:embed lua/touch_session.lua
	luaTouchSessionCode string

	ErrJWTTokenInvalid        = errors.New("jwt token invalid")
	ErrJWTTokenSessionExpired = errors.New("jwt token session expired")
	ErrJWTBackendUnavailable  = errors.New("jwt session backend unavailable")
)

type RedisHandler struct {
	cmd             redis.Cmdable
	accessTokenKey  []byte
	refreshTokenKey []byte
	atTTL           time.Duration
	rtTTL           time.Duration
	idleTTL         time.Duration
	absoluteTTL     time.Duration
}

func NewRedisHandlerWithConfig(cmd redis.Cmdable, cfg appconfig.JWTConfig) *RedisHandler {
	return &RedisHandler{
		cmd:             cmd,
		accessTokenKey:  []byte(cfg.AccessTokenKey),
		refreshTokenKey: []byte(cfg.RefreshTokenKey),
		atTTL:           cfg.AccessTokenTTL,
		rtTTL:           cfg.RefreshTokenTTL,
		idleTTL:         cfg.IdleTimeout,
		absoluteTTL:     cfg.AbsoluteTimeout,
	}
}

func (h *RedisHandler) ClearToken(ctx *gin.Context) error {
	ctx.Header("x-jwt-token", "")
	ctx.Header("x-refresh-token", "")
	uc, ok := ctx.Get("user")
	if !ok {
		return ErrJWTTokenInvalid
	}
	userClaims, ok := uc.(*UserClaims)
	if !ok {
		return ErrJWTTokenInvalid
	}
	_, err := h.cmd.Eval(ctx.Request.Context(), luaClearTokenCode,
		[]string{h.refreshKey(userClaims.Ssid), h.absoluteKey(userClaims.Ssid), h.key(userClaims.Ssid)},
		int(h.atTTL/time.Second)).Int()
	if err != nil {
		return fmt.Errorf("%w: clear session: %v", ErrJWTBackendUnavailable, err)
	}
	return nil
}

func (h *RedisHandler) setRefreshToken(ctx *gin.Context, ssid string, uid int64, jti string, absoluteExpiresAt time.Time) error {
	now := time.Now()
	expiresAt := now.Add(h.rtTTL)
	if absoluteExpiresAt.Before(expiresAt) {
		expiresAt = absoluteExpiresAt
	}
	if !expiresAt.After(now) {
		return ErrJWTTokenSessionExpired
	}
	rc := RefreshClaims{
		Id: uid, Ssid: ssid, Jti: jti, AbsoluteExpiresAt: absoluteExpiresAt.UnixMilli(),
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(expiresAt)},
	}
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, rc)
	tokenStr, err := refreshToken.SignedString(h.refreshTokenKey)
	if err != nil {
		return err
	}
	ctx.Header("x-refresh-token", tokenStr)
	return nil
}

func (h *RedisHandler) storeSession(ctx *gin.Context, ssid, jti string, absoluteExpiresAt time.Time) error {
	absoluteTTL := time.Until(absoluteExpiresAt)
	if absoluteTTL <= 0 {
		return ErrJWTTokenSessionExpired
	}
	idleTTL := minDuration(h.idleTTL, absoluteTTL)
	_, err := h.cmd.Eval(ctx.Request.Context(), luaCreateSessionCode,
		[]string{h.refreshKey(ssid), h.absoluteKey(ssid), h.key(ssid)},
		jti, idleTTL.Milliseconds(), absoluteTTL.Milliseconds(), absoluteExpiresAt.UnixMilli()).Int()
	if err != nil {
		return fmt.Errorf("%w: create session: %v", ErrJWTBackendUnavailable, err)
	}
	return nil
}

func (h *RedisHandler) SetJWTToken(ctx *gin.Context, ssid string, uid int64) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		Id: uid, Ssid: ssid, UserAgent: ctx.GetHeader("User-Agent"),
		RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(h.atTTL))},
	})
	tokenStr, err := token.SignedString(h.accessTokenKey)
	if err != nil {
		return err
	}
	ctx.Header("x-jwt-token", tokenStr)
	return nil
}

func (h *RedisHandler) SetLoginToken(ctx *gin.Context, uid int64) error {
	ssid := uuid.NewString()
	jti := uuid.NewString()
	absoluteExpiresAt := time.Now().Add(h.absoluteTTL)
	if err := h.storeSession(ctx, ssid, jti, absoluteExpiresAt); err != nil {
		return err
	}
	if err := h.SetJWTToken(ctx, ssid, uid); err != nil {
		return err
	}
	return h.setRefreshToken(ctx, ssid, uid, jti, absoluteExpiresAt)
}

func (h *RedisHandler) CheckSession(ctx *gin.Context, ssid string) error {
	res, err := h.cmd.Eval(ctx.Request.Context(), luaTouchSessionCode,
		[]string{h.key(ssid), h.refreshKey(ssid), h.absoluteKey(ssid)}, h.idleTTL.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("%w: check session: %v", ErrJWTBackendUnavailable, err)
	}
	if res != 1 {
		return ErrJWTTokenSessionExpired
	}
	return nil
}

func (h *RedisHandler) key(ssid string) string {
	return fmt.Sprintf("user:ssid:%s", ssid)
}

func (h *RedisHandler) refreshKey(ssid string) string {
	return fmt.Sprintf("user:refresh:ssid:%s", ssid)
}

func (h *RedisHandler) absoluteKey(ssid string) string {
	return fmt.Sprintf("user:session:absolute:ssid:%s", ssid)
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

func (h *RedisHandler) ParseAccessToken(tokenStr string) (*UserClaims, error) {
	if tokenStr == "" {
		return nil, ErrJWTTokenInvalid
	}
	uc := UserClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, &uc, func(token *jwt.Token) (interface{}, error) {
		return h.accessTokenKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		return nil, ErrJWTTokenInvalid
	}
	return &uc, nil
}

func (h *RedisHandler) Refresh(ctx *gin.Context) error {
	tokenStr := ctx.GetHeader("x-refresh-token")
	if tokenStr == "" {
		return ErrJWTTokenInvalid
	}
	var rc RefreshClaims
	token, err := jwt.ParseWithClaims(tokenStr, &rc, func(token *jwt.Token) (interface{}, error) {
		return h.refreshTokenKey, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid || rc.Id <= 0 || rc.Ssid == "" || rc.Jti == "" || rc.AbsoluteExpiresAt <= time.Now().UnixMilli() {
		return ErrJWTTokenInvalid
	}
	newJTI := uuid.NewString()
	res, err := h.cmd.Eval(ctx.Request.Context(), luaRotateRefreshCode,
		[]string{h.refreshKey(rc.Ssid), h.absoluteKey(rc.Ssid), h.key(rc.Ssid)},
		rc.Jti, newJTI, h.idleTTL.Milliseconds()).Int()
	if err != nil {
		return fmt.Errorf("%w: rotate refresh token: %v", ErrJWTBackendUnavailable, err)
	}
	if res != 1 {
		return ErrJWTTokenInvalid
	}
	if err = h.SetJWTToken(ctx, rc.Ssid, rc.Id); err != nil {
		return err
	}
	return h.setRefreshToken(ctx, rc.Ssid, rc.Id, newJTI, time.UnixMilli(rc.AbsoluteExpiresAt))
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}
