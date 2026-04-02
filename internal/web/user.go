package web

import (
	"errors"
	"user-center/internal/domain"
	"user-center/internal/service"
	jwt2 "user-center/internal/web/jwt"

	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
)

const (
	emailRegexPattern    = "^\\w+([-+.]\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$"
	passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`
	bizLogin             = "login"
)

type UserHandler struct {
	svc              service.UserService
	codeSvc          service.CodeService
	emailRegexExp    *regexp.Regexp
	passwordRegexExp *regexp.Regexp
	jwt2.Handler
}

func NewUserHandler(svc service.UserService, codeSvc service.CodeService, jwtHdl jwt2.Handler) *UserHandler {
	return &UserHandler{
		svc:              svc,
		codeSvc:          codeSvc,
		Handler:          jwtHdl,
		emailRegexExp:    regexp.MustCompile(emailRegexPattern, regexp.None),
		passwordRegexExp: regexp.MustCompile(passwordRegexPattern, regexp.None),
	}
}

func (u *UserHandler) RegisterRoutes(server *gin.Engine) {
	ug := server.Group("/user")
	ug.GET("/profile", u.Profile)
	ug.POST("/signup", u.Signup)
	ug.POST("/login", u.Login)
	ug.POST("/logout", u.Logout)
	ug.POST("/edit", u.Edit)
	ug.POST("/login_sms", u.LoginSMS)
	ug.POST("/login_sms/code/send", u.SendSMSLoginCode)
	ug.POST("/refresh_token", u.RefreshToken)
}

func (u *UserHandler) Logout(ctx *gin.Context) {
	err := u.ClearToken(ctx)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
	}
	JSONOK(ctx, "退出登录", nil)
}

func (u *UserHandler) RefreshToken(ctx *gin.Context) {
	err := u.Refresh(ctx)
	if err != nil {
		JSONUnauthorized(ctx, "请登录")
		return
	}
	JSONOK(ctx, "刷新成功", nil)
}

func (u *UserHandler) LoginSMS(ctx *gin.Context) {
	type Req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	var req Req
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	ok, err := u.codeSvc.Verify(ctx.Request.Context(), bizLogin, req.Phone, req.Code)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	if !ok {
		JSONUnauthorized(ctx, "验证码错误")
		return
	}
	user, err := u.svc.FindOrCreate(ctx.Request.Context(), req.Phone)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	err = u.SetLoginToken(ctx, user.Id)
	if err != nil {
		JSONInternalServerError(ctx, "系统异常")
		return
	}
	JSONOK(ctx, "登录成功", nil)
}

func (u *UserHandler) SendSMSLoginCode(ctx *gin.Context) {
	type Req struct {
		Phone string `json:"phone"`
	}
	var req Req
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	if req.Phone == "" {
		JSONBizError(ctx, "请输入手机号")
		return
	}
	err := u.codeSvc.Send(ctx.Request.Context(), bizLogin, req.Phone)
	switch err {
	case nil:
		JSONOK(ctx, "发送成功", nil)
	case service.ErrCodeSendTooMany:
		JSONBizError(ctx, "发送太频繁，请稍候再试")
		return
	default:
		JSONInternalServerError(ctx, "系统错误")
		return
	}
}

func (u *UserHandler) Signup(ctx *gin.Context) {
	type SignUpReq struct {
		Password          string `json:"password"`
		ConfirmedPassword string `json:"confirmed_password"`
		Email             string `json:"email"`
	}
	var req SignUpReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	//邮箱与密码匹配
	isEmail, err := u.emailRegexExp.MatchString(req.Email)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	if !isEmail {
		JSONBizError(ctx, "邮箱格式错误")
		return
	}
	if req.Password != req.ConfirmedPassword {
		JSONBizError(ctx, "两次输入的密码不一致")
		return
	}
	isPassword, err := u.passwordRegexExp.MatchString(req.Password)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	if !isPassword {
		JSONBizError(ctx, "密码必须包含数字、特殊字符，并且长度不能小于 8 位")
		return
	}
	err = u.svc.SignUp(ctx.Request.Context(), domain.User{
		Email:    req.Email,
		Password: req.Password,
	})
	if errors.Is(err, service.ErrUserDuplicate) {
		JSONBizError(ctx, "邮箱已存在")
		return
	}
	if err != nil {
		JSONInternalServerError(ctx, "系统错误,注册失败")
		return
	}
	JSONOK(ctx, "注册成功", nil)
}

func (u *UserHandler) Profile(ctx *gin.Context) {
	val, ok := ctx.Get("user")
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	uc := val.(jwt2.UserClaims)
	user, err := u.svc.Profile(ctx.Request.Context(), uc.Id)
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{
		"id":    user.Id,
		"phone": user.Phone,
		"email": user.Email,
	})
}

func (u *UserHandler) Edit(ctx *gin.Context) {

}

func (u *UserHandler) Login(ctx *gin.Context) {
	type LogInReq struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	var req LogInReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	user, err := u.svc.Login(ctx.Request.Context(),
		req.Email, req.Password,
	)
	if errors.Is(err, service.ErrInvalidUserOrPassword) {
		JSONBizError(ctx, "邮箱或密码不正确")
		return
	}
	if err != nil {
		JSONInternalServerError(ctx, "系统错误")
		return
	}
	err = u.SetLoginToken(ctx, user.Id)
	if err != nil {
		JSONInternalServerError(ctx, "系统异常")
		return
	}
	JSONOK(ctx, "登录成功", nil)
}
