package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	appconfig "user-center/internal/config"
	"user-center/internal/domain"
	jwt2 "user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type wechatServiceStub struct {
	authURLFn    func(ctx context.Context, state string) (string, error)
	verifyCodeFn func(ctx context.Context, code string) (domain.SocialAccount, error)
}

func (s *wechatServiceStub) AuthURL(ctx context.Context, state string) (string, error) {
	if s.authURLFn == nil {
		return "", nil
	}
	return s.authURLFn(ctx, state)
}

func (s *wechatServiceStub) VerifyCode(ctx context.Context, code string) (domain.SocialAccount, error) {
	if s.verifyCodeFn == nil {
		return domain.SocialAccount{}, nil
	}
	return s.verifyCodeFn(ctx, code)
}

type jwtHandlerStubforWeChat struct {
	setLoginTokenFn func(ctx *gin.Context, uid int64) error
}

func (s *jwtHandlerStubforWeChat) ClearToken(ctx *gin.Context) error { return nil }
func (s *jwtHandlerStubforWeChat) SetJWTToken(ctx *gin.Context, ssid string, uid int64) error {
	return nil
}
func (s *jwtHandlerStubforWeChat) SetLoginToken(ctx *gin.Context, uid int64) error {
	if s.setLoginTokenFn == nil {
		ctx.Header("x-jwt-token", "stub-jwt-token")
		return nil
	}
	return s.setLoginTokenFn(ctx, uid)
}
func (s *jwtHandlerStubforWeChat) ExtractAccessTokenString(ctx *gin.Context) string { return "" }
func (s *jwtHandlerStubforWeChat) CheckSession(ctx *gin.Context, ssid string) error { return nil }
func (s *jwtHandlerStubforWeChat) Refresh(ctx *gin.Context) error                   { return nil }
func (s *jwtHandlerStubforWeChat) ParseAccessToken(tokenStr string) (*jwt2.UserClaims, error) {
	return nil, errors.New("not implemented")
}

func testWechatConfig() appconfig.WechatConfig {
	return appconfig.WechatConfig{
		AppID:           "wechat-app-id",
		AppKey:          "wechat-app-key",
		RedirectURL:     "http://localhost:8081/oauth2/wechat/callback",
		StateCookieName: "jwt-state",
		StateTokenKey:   "wechat-state-key",
		StateTokenTTL:   10 * time.Minute,
		StateCookiePath: "/oauth2/wechat/callback",
	}
}

func TestOAuth2WechatHandler_OAuth2URL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testWechatConfig()

	t.Run("success", func(t *testing.T) {
		server := gin.New()
		h := NewOAuth2WechatHandlerWithConfig(&wechatServiceStub{
			authURLFn: func(ctx context.Context, state string) (string, error) {
				if state == "" {
					t.Fatal("state should not be empty")
				}
				return "https://wechat.example.com/auth?state=" + state, nil
			},
		}, &userServiceStub{}, &jwtHandlerStubforWeChat{}, cfg)
		h.RegisterRoutes(server)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/wechat/authurl", nil)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("want status 200, got %d", resp.Code)
		}
		var result Result
		if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if result.Code != CodeSuccess {
			t.Fatalf("unexpected result: %+v", result)
		}
		gotURL, ok := result.Data.(string)
		if !ok || gotURL == "" {
			t.Fatalf("want auth url in response data, got %#v", result.Data)
		}
		cookies := resp.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatalf("want 1 cookie, got %d", len(cookies))
		}
		if cookies[0].Name != cfg.StateCookieName {
			t.Fatalf("unexpected cookie name: %s", cookies[0].Name)
		}
	})

	t.Run("wechat service error", func(t *testing.T) {
		server := gin.New()
		h := NewOAuth2WechatHandlerWithConfig(&wechatServiceStub{
			authURLFn: func(ctx context.Context, state string) (string, error) {
				return "", errors.New("wechat down")
			},
		}, &userServiceStub{}, &jwtHandlerStubforWeChat{}, cfg)
		h.RegisterRoutes(server)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/wechat/authurl", nil)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("want status 500, got %d", resp.Code)
		}
	})
}

func TestOAuth2WechatHandler_CallBack(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cfg := testWechatConfig()

	newStateCookie := func(t *testing.T, state string) *http.Cookie {
		t.Helper()
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, StateClaims{
			State: state,
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(cfg.StateTokenTTL)),
			},
		})
		tokenStr, err := token.SignedString([]byte(cfg.StateTokenKey))
		if err != nil {
			t.Fatalf("sign state cookie: %v", err)
		}
		return &http.Cookie{
			Name:  cfg.StateCookieName,
			Value: tokenStr,
			Path:  cfg.StateCookiePath,
		}
	}

	t.Run("success", func(t *testing.T) {
		server := gin.New()
		h := NewOAuth2WechatHandlerWithConfig(&wechatServiceStub{
			verifyCodeFn: func(ctx context.Context, code string) (domain.SocialAccount, error) {
				if code != "code-123" {
					t.Fatalf("unexpected code: %s", code)
				}
				return domain.SocialAccount{
					OpenId:   "openid-1",
					UnionId:  "union-1",
					Provider: domain.OAuthProviderWechat,
				}, nil
			},
		}, &userServiceStub{
			findByWechatFn: func(ctx context.Context, info domain.SocialAccount) (domain.User, error) {
				if info.OpenId != "openid-1" || info.UnionId != "union-1" {
					t.Fatalf("unexpected social account: %+v", info)
				}
				return domain.User{Id: 1001}, nil
			},
		}, &jwtHandlerStubforWeChat{
			setLoginTokenFn: func(ctx *gin.Context, uid int64) error {
				if uid != 1001 {
					t.Fatalf("unexpected user id: %d", uid)
				}
				ctx.Header("x-jwt-token", "stub-jwt-token")
				return nil
			},
		}, cfg)
		h.RegisterRoutes(server)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/wechat/callback?state=good-state&code=code-123", nil)
		req.AddCookie(newStateCookie(t, "good-state"))
		req.Header.Set("User-Agent", "wechat-unit-test")
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)

		if resp.Code != http.StatusOK {
			t.Fatalf("want status 200, got %d, body=%s", resp.Code, resp.Body.String())
		}
		if resp.Header().Get("x-jwt-token") == "" {
			t.Fatal("jwt token should be set after successful wechat login")
		}
		var result Result
		if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		if result.Code != CodeSuccess || result.Msg != "登录成功" {
			t.Fatalf("unexpected response: %+v", result)
		}
	})

	t.Run("state mismatch", func(t *testing.T) {
		server := gin.New()
		h := NewOAuth2WechatHandlerWithConfig(&wechatServiceStub{
			verifyCodeFn: func(ctx context.Context, code string) (domain.SocialAccount, error) {
				t.Fatal("VerifyCode should not be called when state verification fails")
				return domain.SocialAccount{}, nil
			},
		}, &userServiceStub{}, &jwtHandlerStubforWeChat{}, cfg)
		h.RegisterRoutes(server)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/wechat/callback?state=bad-state&code=code", nil)
		req.AddCookie(newStateCookie(t, "good-state"))
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("want status 500, got %d", resp.Code)
		}
	})

	t.Run("verify code error", func(t *testing.T) {
		server := gin.New()
		h := NewOAuth2WechatHandlerWithConfig(&wechatServiceStub{
			verifyCodeFn: func(ctx context.Context, code string) (domain.SocialAccount, error) {
				return domain.SocialAccount{}, errors.New("wechat verify failed")
			},
		}, &userServiceStub{}, &jwtHandlerStubforWeChat{}, cfg)
		h.RegisterRoutes(server)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/wechat/callback?state=good-state&code=code", nil)
		req.AddCookie(newStateCookie(t, "good-state"))
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("want status 500, got %d", resp.Code)
		}
	})

	t.Run("user service error", func(t *testing.T) {
		server := gin.New()
		h := NewOAuth2WechatHandlerWithConfig(&wechatServiceStub{
			verifyCodeFn: func(ctx context.Context, code string) (domain.SocialAccount, error) {
				return domain.SocialAccount{OpenId: "openid"}, nil
			},
		}, &userServiceStub{
			findByWechatFn: func(ctx context.Context, info domain.SocialAccount) (domain.User, error) {
				return domain.User{}, errors.New("create user failed")
			},
		}, &jwtHandlerStubforWeChat{}, cfg)
		h.RegisterRoutes(server)

		req := httptest.NewRequest(http.MethodGet, "/oauth2/wechat/callback?state=good-state&code=code", nil)
		req.AddCookie(newStateCookie(t, "good-state"))
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)

		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("want status 500, got %d", resp.Code)
		}
	})
}
