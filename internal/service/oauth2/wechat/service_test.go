package wechat

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"user-center/internal/domain"
)

type roundTripFunc func(req *http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestService_AuthURL(t *testing.T) {
	t.Parallel()

	svc := &service{appId: "appid-123"}
	got, err := svc.AuthURL(context.Background(), "state-xyz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "https://open.weixin.qq.com/connect/qrconnect?appid=appid-123&redirect_uri=" + redirectURL + "&response_type=code&scope=snsapi_login&state=state-xyz#wechat_redirect"
	if got != want {
		t.Fatalf("want %s, got %s", want, got)
	}
}

func TestService_VerifyCode(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		svc := &service{
			appId:     "appid-123",
			appSecret: "secret-456",
			client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				if req.Method != http.MethodGet {
					t.Fatalf("want GET request, got %s", req.Method)
				}
				if req.URL.Scheme != "https" || req.URL.Host != "api.weixin.qq.com" || req.URL.Path != "/sns/oauth2/access_token" {
					t.Fatalf("unexpected verify url: %s", req.URL.String())
				}
				query := req.URL.Query()
				if query.Get("appid") != "appid-123" || query.Get("secret") != "secret-456" || query.Get("code") != "code-789" || query.Get("grant_type") != "authorization_code" {
					t.Fatalf("unexpected query params: %v", query)
				}
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"openid":"openid-1","unionid":"union-1"}`)),
					Header:     make(http.Header),
				}, nil
			})},
		}

		got, err := svc.VerifyCode(context.Background(), "code-789")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := domain.SocialAccount{
			OpenId:   "openid-1",
			UnionId:  "union-1",
			Provider: domain.OAuthProviderWechat,
		}
		if got != want {
			t.Fatalf("want %+v, got %+v", want, got)
		}
	})

	t.Run("wechat returns business error", func(t *testing.T) {
		t.Parallel()
		svc := &service{
			client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{"errcode":40029,"errmsg":"invalid code"}`)),
					Header:     make(http.Header),
				}, nil
			})},
		}

		_, err := svc.VerifyCode(context.Background(), "bad-code")
		if err == nil || err.Error() != "换取 access_token 失败" {
			t.Fatalf("want business error, got %v", err)
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		t.Parallel()
		svc := &service{
			client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`not-json`)),
					Header:     make(http.Header),
				}, nil
			})},
		}

		_, err := svc.VerifyCode(context.Background(), "code")
		if err == nil {
			t.Fatal("want json decode error, got nil")
		}
	})

	t.Run("http client error", func(t *testing.T) {
		t.Parallel()
		svc := &service{
			client: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
				return nil, errors.New("network down")
			})},
		}

		_, err := svc.VerifyCode(context.Background(), "code")
		if err == nil || !strings.Contains(err.Error(), "network down") {
			t.Fatalf("want error contains network down, got %v", err)
		}
	})
}
