package wechat

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"user-center/internal/domain"
)

const authURLPattern = "https://open.weixin.qq.com/connect/qrconnect?appid=%s&redirect_uri=%s&response_type=code&scope=snsapi_login&state=%s#wechat_redirect"

type service struct {
	appId       string
	appSecret   string
	redirectURL string
	client      *http.Client
}

func NewService(appId string, appSecret string, redirect string) Service {
	escaped := url.PathEscape(redirect)
	return &service{
		appId:       appId,
		appSecret:   appSecret,
		redirectURL: escaped,
		client:      &http.Client{},
	}
}

func (s *service) AuthURL(ctx context.Context, state string) (string, error) {
	redirect := s.redirectURL
	return fmt.Sprintf(authURLPattern, s.appId, redirect, state), nil
}

func (s *service) VerifyCode(ctx context.Context, code string) (domain.SocialAccount, error) {
	const baseURL = "https://api.weixin.qq.com/sns/oauth2/access_token"
	queryParams := url.Values{}
	queryParams.Set("appid", s.appId)
	queryParams.Set("secret", s.appSecret)
	queryParams.Set("code", code)
	queryParams.Set("grant_type", "authorization_code")
	accessTokenURL := baseURL + "?" + queryParams.Encode()
	req, err := http.NewRequest("GET", accessTokenURL, nil)
	if err != nil {
		return domain.SocialAccount{}, err
	}
	req = req.WithContext(ctx)
	resp, err := s.client.Do(req)
	if err != nil {
		return domain.SocialAccount{}, err
	}
	defer resp.Body.Close()
	var res Result

	err = json.NewDecoder(resp.Body).Decode(&res)
	if err != nil {
		return domain.SocialAccount{}, err
	}
	if res.ErrCode != 0 {
		return domain.SocialAccount{}, errors.New("换取 access_token 失败")
	}
	return domain.SocialAccount{
		OpenId:   res.OpenId,
		UnionId:  res.UnionId,
		Provider: domain.OAuthProviderWechat,
	}, nil
}

type Result struct {
	ErrCode int64  `json:"errcode"`
	ErrMsg  string `json:"errMsg"`

	Scope string `json:"scope"`

	AccessToken  string `json:"access_token"`
	ExpiresIn    int64  `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`

	OpenId  string `json:"openid"`
	UnionId string `json:"unionid"`
}
