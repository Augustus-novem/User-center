package ioc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	appconfig "user-center/internal/config"
	"user-center/internal/web"

	"github.com/gin-gonic/gin"
)

type dynamicProviderStub struct {
	dynamic appconfig.DynamicConfig
}

func (d *dynamicProviderStub) Dynamic() appconfig.DynamicConfig {
	return d.dynamic
}

func TestFeatureGuardMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		path       string
		dynamic    appconfig.DynamicConfig
		wantStatus int
		wantCode   int
		wantMsg    string
		reached    bool
	}{
		{
			name: "wechat login disabled",
			path: "/oauth2/wechat/authurl",
			dynamic: appconfig.DynamicConfig{Feature: appconfig.FeatureConfig{
				EnableWechatLogin: false,
			}},
			wantStatus: http.StatusOK,
			wantCode:   web.CodeBadRequest,
			wantMsg:    "微信登录功能未开启",
		},
		{
			name: "sms login disabled",
			path: "/user/login_sms",
			dynamic: appconfig.DynamicConfig{Feature: appconfig.FeatureConfig{
				EnableWechatLogin: true,
				EnableSMSLogin:    false,
			}},
			wantStatus: http.StatusOK,
			wantCode:   web.CodeBadRequest,
			wantMsg:    "短信登录功能未开启",
		},
		{
			name: "feature enabled request passes",
			path: "/oauth2/wechat/authurl",
			dynamic: appconfig.DynamicConfig{Feature: appconfig.FeatureConfig{
				EnableWechatLogin: true,
				EnableSMSLogin:    true,
			}},
			wantStatus: http.StatusOK,
			wantCode:   web.CodeSuccess,
			wantMsg:    "ok",
			reached:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := gin.New()
			server.Use(featureGuardMiddleware(&dynamicProviderStub{dynamic: tc.dynamic}))
			server.GET(tc.path, func(ctx *gin.Context) {
				web.JSONOK(ctx, "ok", nil)
			})

			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			if resp.Code != tc.wantStatus {
				t.Fatalf("want status %d, got %d", tc.wantStatus, resp.Code)
			}
			var result web.Result
			if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if result.Code != tc.wantCode || result.Msg != tc.wantMsg {
				t.Fatalf("want result code=%d msg=%q, got %+v", tc.wantCode, tc.wantMsg, result)
			}
		})
	}
}
