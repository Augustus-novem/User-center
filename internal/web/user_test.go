package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/service"
	jwt2 "user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

type userServiceStub struct {
	loginFn                  func(ctx context.Context, email, password string) (domain.User, error)
	findOrCreateFn           func(ctx context.Context, phone string) (domain.User, error)
	findByWechatFn           func(ctx context.Context, info domain.SocialAccount) (domain.User, error)
	signUpFn                 func(ctx context.Context, user domain.User) error
	profileFn                func(ctx context.Context, id int64) (domain.User, error)
	updateNonSensitiveInfoFn func(ctx context.Context, user domain.User) error
}

func (s *userServiceStub) Login(ctx context.Context, email, password string) (domain.User, error) {
	if s.loginFn == nil {
		return domain.User{}, nil
	}
	return s.loginFn(ctx, email, password)
}

func (s *userServiceStub) FindOrCreate(ctx context.Context, phone string) (domain.User, error) {
	if s.findOrCreateFn == nil {
		return domain.User{}, nil
	}
	return s.findOrCreateFn(ctx, phone)
}

func (s *userServiceStub) FindOrCreateByWechat(ctx context.Context, info domain.SocialAccount) (domain.User, error) {
	if s.findByWechatFn == nil {
		return domain.User{}, nil
	}
	return s.findByWechatFn(ctx, info)
}

func (s *userServiceStub) SignUp(ctx context.Context, user domain.User) error {
	if s.signUpFn == nil {
		return nil
	}
	return s.signUpFn(ctx, user)
}

func (s *userServiceStub) Profile(ctx context.Context, id int64) (domain.User, error) {
	if s.profileFn == nil {
		return domain.User{}, nil
	}
	return s.profileFn(ctx, id)
}

func (s *userServiceStub) UpdateNonSensitiveInfo(ctx context.Context, user domain.User) error {
	if s.updateNonSensitiveInfoFn == nil {
		return nil
	}
	return s.updateNonSensitiveInfoFn(ctx, user)
}

type codeServiceStub struct {
	sendFn   func(ctx context.Context, biz, phone string) error
	verifyFn func(ctx context.Context, biz, phone, inputCode string) (bool, error)
}

func (s *codeServiceStub) Send(ctx context.Context, biz, phone string) error {
	if s.sendFn == nil {
		return nil
	}
	return s.sendFn(ctx, biz, phone)
}

func (s *codeServiceStub) Verify(ctx context.Context, biz, phone, inputCode string) (bool, error) {
	if s.verifyFn == nil {
		return false, nil
	}
	return s.verifyFn(ctx, biz, phone, inputCode)
}

type jwtHandlerStubForUser struct {
	setLoginTokenFn func(ctx *gin.Context, uid int64) error
	clearTokenFn    func(ctx *gin.Context) error
	refreshFn       func(ctx *gin.Context) error
}

func (s *jwtHandlerStubForUser) ClearToken(ctx *gin.Context) error {
	if s.clearTokenFn != nil {
		return s.clearTokenFn(ctx)
	}
	ctx.Header("x-jwt-token", "")
	ctx.Header("x-refresh-token", "")
	return nil
}

func (s *jwtHandlerStubForUser) SetJWTToken(ctx *gin.Context, ssid string, uid int64) error {
	ctx.Header("x-jwt-token", "test-jwt-token")
	return nil
}

func (s *jwtHandlerStubForUser) SetLoginToken(ctx *gin.Context, uid int64) error {
	if s.setLoginTokenFn != nil {
		return s.setLoginTokenFn(ctx, uid)
	}
	ctx.Header("x-jwt-token", "test-jwt-token")
	ctx.Header("x-refresh-token", "test-refresh-token")
	return nil
}

func (s *jwtHandlerStubForUser) ExtractAccessTokenString(ctx *gin.Context) string {
	return ""
}

func (s *jwtHandlerStubForUser) CheckSession(ctx *gin.Context, ssid string) error {
	return nil
}

func (s *jwtHandlerStubForUser) Refresh(ctx *gin.Context) error {
	if s.refreshFn != nil {
		return s.refreshFn(ctx)
	}
	ctx.Header("x-jwt-token", "refreshed-jwt-token")
	ctx.Header("x-refresh-token", "refreshed-refresh-token")
	return nil
}

func (s *jwtHandlerStubForUser) ParseAccessToken(tokenStr string) (*jwt2.UserClaims, error) {
	return &jwt2.UserClaims{Id: 1}, nil
}

func TestUserHandler_Signup(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		userSvc    service.UserService
		wantStatus int
		wantCode   int
		wantMsg    string
	}{
		{
			name: "signup success",
			body: `{"email":"123@qq.com","password":"hello@world123","confirmed_password":"hello@world123"}`,
			userSvc: &userServiceStub{
				signUpFn: func(ctx context.Context, user domain.User) error {
					if user.Email != "123@qq.com" {
						t.Fatalf("unexpected email: %s", user.Email)
					}
					if user.Password != "hello@world123" {
						t.Fatalf("handler should pass raw password to service, got %s", user.Password)
					}
					return nil
				},
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeSuccess,
			wantMsg:    "注册成功",
		},
		{
			name:       "bad json",
			body:       `{"email":`,
			userSvc:    &userServiceStub{},
			wantStatus: http.StatusBadRequest,
			wantCode:   CodeBadRequest,
			wantMsg:    "请求参数错误",
		},
		{
			name:       "invalid email",
			body:       `{"email":"123@","password":"hello@world123","confirmed_password":"hello@world123"}`,
			userSvc:    &userServiceStub{},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "邮箱格式错误",
		},
		{
			name:       "password mismatch",
			body:       `{"email":"123@qq.com","password":"hello@world123","confirmed_password":"another@world123"}`,
			userSvc:    &userServiceStub{},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "两次输入的密码不一致",
		},
		{
			name:       "invalid password format",
			body:       `{"email":"123@qq.com","password":"hello","confirmed_password":"hello"}`,
			userSvc:    &userServiceStub{},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "密码必须包含数字、特殊字符，并且长度不能小于 8 位",
		},
		{
			name: "duplicate email",
			body: `{"email":"123@qq.com","password":"hello@world123","confirmed_password":"hello@world123"}`,
			userSvc: &userServiceStub{
				signUpFn: func(ctx context.Context, user domain.User) error {
					return service.ErrUserDuplicate
				},
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "邮箱已存在",
		},
		{
			name: "system error",
			body: `{"email":"123@qq.com","password":"hello@world123","confirmed_password":"hello@world123"}`,
			userSvc: &userServiceStub{
				signUpFn: func(ctx context.Context, user domain.User) error {
					return errors.New("db down")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternalErr,
			wantMsg:    "系统错误,注册失败",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := gin.New()
			h := NewUserHandler(tc.userSvc, &codeServiceStub{}, &jwtHandlerStubForUser{})
			h.RegisterRoutes(server)

			req := httptest.NewRequest(http.MethodPost, "/user/signup", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			server.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("want status %d, got %d", tc.wantStatus, resp.Code)
			}
			var result Result
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if result.Code != tc.wantCode || result.Msg != tc.wantMsg {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}

func TestUserHandler_Login(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		userSvc    service.UserService
		wantStatus int
		wantCode   int
		wantMsg    string
		wantJWT    bool
	}{
		{
			name: "login success",
			body: `{"email":"123@qq.com","password":"hello@world123"}`,
			userSvc: &userServiceStub{
				loginFn: func(ctx context.Context, email, password string) (domain.User, error) {
					return domain.User{Id: 11, Email: email}, nil
				},
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeSuccess,
			wantMsg:    "登录成功",
			wantJWT:    true,
		},
		{
			name: "invalid user or password",
			body: `{"email":"123@qq.com","password":"bad"}`,
			userSvc: &userServiceStub{
				loginFn: func(ctx context.Context, email, password string) (domain.User, error) {
					return domain.User{}, service.ErrInvalidUserOrPassword
				},
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "邮箱或密码不正确",
		},
		{
			name: "system error",
			body: `{"email":"123@qq.com","password":"bad"}`,
			userSvc: &userServiceStub{
				loginFn: func(ctx context.Context, email, password string) (domain.User, error) {
					return domain.User{}, errors.New("db down")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternalErr,
			wantMsg:    "系统错误",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := gin.New()
			h := NewUserHandler(tc.userSvc, &codeServiceStub{}, &jwtHandlerStubForUser{})
			h.RegisterRoutes(server)

			req := httptest.NewRequest(http.MethodPost, "/user/login", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("User-Agent", "unit-test-agent")
			resp := httptest.NewRecorder()

			server.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("want status %d, got %d", tc.wantStatus, resp.Code)
			}
			var result Result
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if result.Code != tc.wantCode || result.Msg != tc.wantMsg {
				t.Fatalf("unexpected result: %+v", result)
			}
			gotToken := resp.Header().Get("x-jwt-token") != ""
			if gotToken != tc.wantJWT {
				t.Fatalf("want jwt=%v, got jwt=%v", tc.wantJWT, gotToken)
			}
		})
	}
}

func TestUserHandler_SendSMSLoginCode(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		body       string
		codeSvc    service.CodeService
		wantStatus int
		wantCode   int
		wantMsg    string
	}{
		{
			name: "send success",
			body: `{"phone":"13800138000"}`,
			codeSvc: &codeServiceStub{
				sendFn: func(ctx context.Context, biz, phone string) error {
					if biz != bizLogin || phone != "13800138000" {
						t.Fatalf("unexpected biz/phone: %s %s", biz, phone)
					}
					return nil
				},
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeSuccess,
			wantMsg:    "发送成功",
		},
		{
			name:       "empty phone",
			body:       `{"phone":""}`,
			codeSvc:    &codeServiceStub{},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "请输入手机号",
		},
		{
			name: "send too frequently",
			body: `{"phone":"13800138000"}`,
			codeSvc: &codeServiceStub{
				sendFn: func(ctx context.Context, biz, phone string) error {
					return service.ErrCodeSendTooMany
				},
			},
			wantStatus: http.StatusOK,
			wantCode:   CodeBadRequest,
			wantMsg:    "发送太频繁，请稍候再试",
		},
		{
			name: "system error",
			body: `{"phone":"13800138000"}`,
			codeSvc: &codeServiceStub{
				sendFn: func(ctx context.Context, biz, phone string) error {
					return errors.New("redis down")
				},
			},
			wantStatus: http.StatusInternalServerError,
			wantCode:   CodeInternalErr,
			wantMsg:    "系统错误",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := gin.New()
			h := NewUserHandler(&userServiceStub{}, tc.codeSvc, &jwtHandlerStubForUser{})
			h.RegisterRoutes(server)

			req := httptest.NewRequest(http.MethodPost, "/user/login_sms/code/send", bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()

			server.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("want status %d, got %d", tc.wantStatus, resp.Code)
			}
			var result Result
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatalf("failed to unmarshal response: %v", err)
			}
			if result.Code != tc.wantCode || result.Msg != tc.wantMsg {
				t.Fatalf("unexpected result: %+v", result)
			}
		})
	}
}
