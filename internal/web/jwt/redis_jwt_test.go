package jwt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
	appconfig "user-center/internal/config"

	"github.com/gin-gonic/gin"
	gjwt "github.com/golang-jwt/jwt/v5"
	"github.com/redis/go-redis/v9"
)

func testJWTConfig() appconfig.JWTConfig {
	return appconfig.JWTConfig{
		AccessTokenKey:  "access-secret-for-test",
		RefreshTokenKey: "refresh-secret-for-test",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 24 * time.Hour,
		IdleTimeout:     24 * time.Hour,
		AbsoluteTimeout: 7 * 24 * time.Hour,
	}
}

type fakeRedisValue struct {
	val      string
	expireAt time.Time
}

type fakeRedis struct {
	// 嵌一个 redis.Cmdable，借它“满足接口”；
	// 我们只覆写这几个测试实际会调用的方法。
	redis.Cmdable

	mu    sync.Mutex
	store map[string]fakeRedisValue
}

func newFakeRedis() *fakeRedis {
	return &fakeRedis{
		store: make(map[string]fakeRedisValue),
	}
}

func (f *fakeRedis) set(key, val string, ttl time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.store[key] = fakeRedisValue{
		val:      val,
		expireAt: time.Now().Add(ttl),
	}
}

func (f *fakeRedis) get(key string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	v, ok := f.store[key]
	if !ok {
		return "", false
	}
	if time.Now().After(v.expireAt) {
		delete(f.store, key)
		return "", false
	}
	return v.val, true
}

func (f *fakeRedis) del(key string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.store, key)
}

func (f *fakeRedis) ttl(key string) (time.Duration, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	v, ok := f.store[key]
	if !ok {
		return 0, false
	}
	left := time.Until(v.expireAt)
	if left <= 0 {
		delete(f.store, key)
		return 0, false
	}
	return left, true
}

func (f *fakeRedis) Exists(ctx context.Context, keys ...string) *redis.IntCmd {
	var cnt int64
	for _, key := range keys {
		if _, ok := f.get(key); ok {
			cnt++
		}
	}
	return redis.NewIntResult(cnt, nil)
}

func (f *fakeRedis) Eval(ctx context.Context, script string, keys []string, args ...interface{}) *redis.Cmd {
	// 这里只模拟你项目里用到的两段 Lua：
	// 1. clear_token.lua: KEYS=2, ARGS=1
	// 2. compare_and_set.lua: KEYS=1, ARGS=3
	switch {
	case len(keys) == 2 && len(args) == 1:
		// clear_token.lua
		ttlSeconds, ok := toInt(args[0])
		if !ok {
			return redis.NewCmdResult(nil, errors.New("invalid ttl arg"))
		}
		f.del(keys[0])
		f.set(keys[1], "logout", time.Duration(ttlSeconds)*time.Second)
		return redis.NewCmdResult(int64(1), nil)

	case len(keys) == 1 && len(args) == 3:
		// compare_and_set.lua
		oldJTI, ok1 := args[0].(string)
		newJTI, ok2 := args[1].(string)
		ttlSeconds, ok3 := toInt(args[2])
		if !ok1 || !ok2 || !ok3 {
			return redis.NewCmdResult(nil, errors.New("invalid rotation args"))
		}

		cur, ok := f.get(keys[0])
		if !ok {
			return redis.NewCmdResult(int64(0), nil)
		}
		if cur != oldJTI {
			return redis.NewCmdResult(int64(0), nil)
		}
		f.set(keys[0], newJTI, time.Duration(ttlSeconds)*time.Second)
		return redis.NewCmdResult(int64(1), nil)

	default:
		return redis.NewCmdResult(nil, errors.New("unexpected eval call"))
	}
}

func toInt(v interface{}) (int, bool) {
	switch val := v.(type) {
	case int:
		return val, true
	case int64:
		return int(val), true
	case int32:
		return int(val), true
	default:
		return 0, false
	}
}

func newTestRedisHandler(t *testing.T) (*RedisHandler, *fakeRedis) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	cmd := newFakeRedis()
	h := NewRedisHandlerWithConfig(cmd, testJWTConfig())
	return h, cmd
}

func newGinCtx(method, path string) (*gin.Context, *httptest.ResponseRecorder) {
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	req := httptest.NewRequest(method, path, nil)
	ctx.Request = req
	return ctx, recorder
}

func mustRefreshToken(t *testing.T, uid int64, ssid, jti string, ttl time.Duration) string {
	t.Helper()
	token := gjwt.NewWithClaims(gjwt.SigningMethodHS256, RefreshClaims{
		Id:   uid,
		Ssid: ssid,
		Jti:  jti,
		RegisteredClaims: gjwt.RegisteredClaims{
			ExpiresAt: gjwt.NewNumericDate(time.Now().Add(ttl)),
		},
	})
	tokenStr, err := token.SignedString([]byte(testJWTConfig().RefreshTokenKey))
	if err != nil {
		t.Fatalf("sign refresh token: %v", err)
	}
	return tokenStr
}

func mustParseRefreshToken(t *testing.T, tokenStr string) RefreshClaims {
	t.Helper()
	var rc RefreshClaims
	token, err := gjwt.ParseWithClaims(tokenStr, &rc, func(token *gjwt.Token) (interface{}, error) {
		return []byte(testJWTConfig().RefreshTokenKey), nil
	}, gjwt.WithValidMethods([]string{gjwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		t.Fatalf("parse refresh token failed: %v", err)
	}
	return rc
}

func mustParseAccessToken(t *testing.T, tokenStr string) UserClaims {
	t.Helper()
	var uc UserClaims
	token, err := gjwt.ParseWithClaims(tokenStr, &uc, func(token *gjwt.Token) (interface{}, error) {
		return []byte(testJWTConfig().AccessTokenKey), nil
	}, gjwt.WithValidMethods([]string{gjwt.SigningMethodHS256.Alg()}))
	if err != nil || token == nil || !token.Valid {
		t.Fatalf("parse access token failed: %v", err)
	}
	return uc
}

func TestRedisHandler_CheckSession(t *testing.T) {
	h, fake := newTestRedisHandler(t)
	ctx, _ := newGinCtx(http.MethodGet, "/user/profile")
	ssid := "check-session-ssid"

	if err := h.CheckSession(ctx, ssid); err != nil {
		t.Fatalf("not logged out yet, should pass, got err=%v", err)
	}

	fake.set(h.key(ssid), "logout", time.Minute)
	err := h.CheckSession(ctx, ssid)
	if err == nil {
		t.Fatal("want logged-out error, got nil")
	}
}

func TestRedisHandler_ClearToken(t *testing.T) {
	h, fake := newTestRedisHandler(t)
	ctx, recorder := newGinCtx(http.MethodPost, "/user/logout")
	ssid := "logout-ssid"

	fake.set(h.refreshKey(ssid), "old-jti", h.rtTTL)
	ctx.Set("user", UserClaims{Id: 123, Ssid: ssid})

	if err := h.ClearToken(ctx); err != nil {
		t.Fatalf("clear token failed: %v", err)
	}

	if got := recorder.Header().Get("x-jwt-token"); got != "" {
		t.Fatalf("want empty x-jwt-token, got %q", got)
	}
	// 注意：这里按你当前生产代码里的拼写来断言
	if got := recorder.Header().Get("x-refresh-token"); got != "" {
		t.Fatalf("want empty x-refresh-token, got %q", got)
	}

	if _, ok := fake.get(h.refreshKey(ssid)); ok {
		t.Fatal("refresh key should be deleted after logout")
	}

	val, ok := fake.get(h.key(ssid))
	if !ok {
		t.Fatal("logout key should exist")
	}
	if val != "logout" {
		t.Fatalf("want logout marker, got %q", val)
	}

	ttl, ok := fake.ttl(h.key(ssid))
	if !ok {
		t.Fatal("logout ttl should exist")
	}
	if ttl <= 0 || ttl > h.atTTL {
		t.Fatalf("unexpected logout ttl: %v", ttl)
	}

	if err := h.CheckSession(ctx, ssid); err == nil {
		t.Fatal("session should be marked as logged out")
	}
}

func TestRedisHandler_Refresh_RotationSuccess(t *testing.T) {
	h, fake := newTestRedisHandler(t)
	ctx, recorder := newGinCtx(http.MethodPost, "/user/refresh_token")
	ctx.Request.Header.Set("User-Agent", "unit-test-agent")

	const (
		uid    = int64(123)
		ssid   = "refresh-ssid"
		oldJTI = "old-jti"
	)
	fake.set(h.refreshKey(ssid), oldJTI, h.rtTTL)

	oldRT := mustRefreshToken(t, uid, ssid, oldJTI, h.rtTTL)
	ctx.Request.Header.Set("x-refresh-token", oldRT)

	if err := h.Refresh(ctx); err != nil {
		t.Fatalf("refresh should succeed, got err=%v", err)
	}

	gotAT := recorder.Header().Get("x-jwt-token")
	if gotAT == "" {
		t.Fatal("want new access token in x-jwt-token")
	}
	gotRT := recorder.Header().Get("x-refresh-token")
	if gotRT == "" {
		t.Fatal("want new refresh token in x-refresh-token")
	}

	newRC := mustParseRefreshToken(t, gotRT)
	if newRC.Id != uid || newRC.Ssid != ssid {
		t.Fatalf("unexpected new refresh claims: %+v", newRC)
	}
	if newRC.Jti == oldJTI {
		t.Fatalf("rotation should issue a new jti, still got old %q", newRC.Jti)
	}

	currentJTI, ok := fake.get(h.refreshKey(ssid))
	if !ok {
		t.Fatal("latest refresh jti should exist in fake redis")
	}
	if currentJTI != newRC.Jti {
		t.Fatalf("fake redis should store latest jti=%q, got %q", newRC.Jti, currentJTI)
	}

	newUC := mustParseAccessToken(t, gotAT)
	if newUC.Id != uid || newUC.Ssid != ssid || newUC.UserAgent != "unit-test-agent" {
		t.Fatalf("unexpected new access claims: %+v", newUC)
	}
}

func TestRedisHandler_Refresh_UsingOldTokenAfterRotationFails(t *testing.T) {
	h, fake := newTestRedisHandler(t)

	const (
		uid    = int64(123)
		ssid   = "refresh-old-token-ssid"
		oldJTI = "old-jti"
	)
	fake.set(h.refreshKey(ssid), oldJTI, h.rtTTL)
	oldRT := mustRefreshToken(t, uid, ssid, oldJTI, h.rtTTL)

	firstCtx, firstRecorder := newGinCtx(http.MethodPost, "/user/refresh_token")
	firstCtx.Request.Header.Set("User-Agent", "unit-test-agent")
	firstCtx.Request.Header.Set("x-refresh-token", oldRT)
	if err := h.Refresh(firstCtx); err != nil {
		t.Fatalf("first refresh should succeed, got err=%v", err)
	}

	latestRT := firstRecorder.Header().Get("x-refresh-token")
	latestClaims := mustParseRefreshToken(t, latestRT)

	secondCtx, _ := newGinCtx(http.MethodPost, "/user/refresh_token")
	secondCtx.Request.Header.Set("User-Agent", "unit-test-agent")
	secondCtx.Request.Header.Set("x-refresh-token", oldRT)
	err := h.Refresh(secondCtx)
	if !errors.Is(err, ErrJWTTokenInvalid) {
		t.Fatalf("old refresh token should be invalid after rotation, got err=%v", err)
	}

	currentJTI, ok := fake.get(h.refreshKey(ssid))
	if !ok {
		t.Fatal("latest refresh jti should still exist")
	}
	if currentJTI != latestClaims.Jti {
		t.Fatalf("fake redis jti should stay at latest=%q, got %q", latestClaims.Jti, currentJTI)
	}
}

func TestRedisHandler_Refresh_FailsAfterLogout(t *testing.T) {
	h, fake := newTestRedisHandler(t)

	const (
		uid  = int64(123)
		ssid = "logout-refresh-ssid"
		jti  = "jti-before-logout"
	)
	fake.set(h.refreshKey(ssid), jti, h.rtTTL)

	logoutCtx, _ := newGinCtx(http.MethodPost, "/user/logout")
	logoutCtx.Set("user", UserClaims{Id: uid, Ssid: ssid})
	if err := h.ClearToken(logoutCtx); err != nil {
		t.Fatalf("clear token: %v", err)
	}

	refreshCtx, _ := newGinCtx(http.MethodPost, "/user/refresh_token")
	refreshCtx.Request.Header.Set("User-Agent", "unit-test-agent")
	refreshCtx.Request.Header.Set("x-refresh-token", mustRefreshToken(t, uid, ssid, jti, h.rtTTL))

	err := h.Refresh(refreshCtx)
	if !errors.Is(err, ErrJWTTokenInvalid) {
		t.Fatalf("refresh after logout should fail, got err=%v", err)
	}
}
