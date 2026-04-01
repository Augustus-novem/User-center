package web

import (
	"errors"
	"fmt"
	"user-center/internal/service"
	"user-center/internal/service/oauth2/wechat"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type OAuth2WechatHandler struct {
	wechatSvc       wechat.Service
	userSvc         service.UserService
	stateCookieName string
	jwtHandler
}

func NewOAuth2WechatHandler(service wechat.Service,
	userSvc service.UserService) *OAuth2WechatHandler {
	return &OAuth2WechatHandler{
		wechatSvc:       service,
		userSvc:         userSvc,
		stateCookieName: "jwt-state",
	}
}

func (h *OAuth2WechatHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/oauth2/wechat")
	g.GET("/authurl", h.OAuth2URL)
	g.Any("/callback", h.CallBack)
}

func (h *OAuth2WechatHandler) OAuth2URL(ctx *gin.Context) {
	state := uuid.New().String()
	url, err := h.wechatSvc.AuthURL(ctx.Request.Context(), state)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	err = h.setStateCookie(ctx, state)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	JSONOK(ctx, "", url)
	return
}

func (h *OAuth2WechatHandler) CallBack(ctx *gin.Context) {
	err := h.verifyState(ctx)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	code := ctx.Query("code")
	info, err := h.wechatSvc.VerifyCode(ctx.Request.Context(), code)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	user, err := h.userSvc.FindOrCreateByWechat(ctx.Request.Context(), info)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	err = h.setJWTToken(ctx, user.Id)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	JSONOK(ctx, "登录成功", "")
}

func (h *OAuth2WechatHandler) verifyState(ctx *gin.Context) error {
	state := ctx.Query("state")
	tokenStr, err := ctx.Cookie(h.stateCookieName)
	if err != nil {
		return err
	}
	var sc StateClaims
	_, err = jwt.ParseWithClaims(tokenStr, &sc, func(token *jwt.Token) (interface{}, error) {
		return JWTKey, nil
	})
	if err != nil {
		return fmt.Errorf("%w, cookie 不是合法 JWT token", err)
	}
	if state != sc.State {
		return errors.New("state 不匹配")
	}
	return nil
}

func (h *OAuth2WechatHandler) setStateCookie(ctx *gin.Context, state string) error {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, StateClaims{
		State: state,
	})
	tokenStr, err := token.SignedString(JWTKey)
	if err != nil {
		return err
	}
	ctx.SetCookie(h.stateCookieName, tokenStr,
		600,
		"/oauth2/wechat/callback",
		"", false, true)
	return nil
}
