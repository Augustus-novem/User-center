package web

import (
	"errors"
	"net/http"
	"time"
	"user-center/internal/domain"
	"user-center/internal/service"

	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

const (
	emailRegexPattern    = "^\\w+([-+.]\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$"
	passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`
	bizLogin             = "login"
)

type UserHandler struct {
	svc              *service.UserService
	codeSvc          *service.CodeService
	emailRegexExp    *regexp.Regexp
	passwordRegexExp *regexp.Regexp
}

func NewUserHandler(svc *service.UserService, codeSvc *service.CodeService) *UserHandler {
	return &UserHandler{
		svc:              svc,
		codeSvc:          codeSvc,
		emailRegexExp:    regexp.MustCompile(emailRegexPattern, regexp.None),
		passwordRegexExp: regexp.MustCompile(passwordRegexPattern, regexp.None),
	}
}

func (u *UserHandler) Register(server *gin.Engine) {
	ug := server.Group("/user")
	ug.GET("/profile", u.Profile)
	ug.POST("/signup", u.Signup)
	ug.POST("/login", u.Login)
	ug.POST("/edit", u.Edit)
	ug.POST("/login_sms", u.LoginSMS)
	ug.POST("/login_sms/code/send", u.SendSMSLoginCode)
}

func (u *UserHandler) LoginSMS(ctx *gin.Context) {
	type Req struct {
		Phone string `json:"phone"`
		Code  string `json:"code"`
	}
	var req Req
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "系统错误0",
		},
		)
		return
	}
	ok, err := u.codeSvc.Verify(ctx.Request.Context(), bizLogin, req.Phone, req.Code)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{
			"error": "系统错误1",
		})
		return
	}
	if !ok {
		ctx.JSON(http.StatusUnauthorized, gin.H{
			"error": "验证码错误",
		})
		return
	}
	user, err := u.svc.FindOrCreate(ctx.Request.Context(), req.Phone)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误2",
		})
		return
	}
	u.setJWTToken(ctx, user.Id)
	ctx.JSON(http.StatusOK, gin.H{
		"msg": "登录成功",
	})
}

func (u *UserHandler) SendSMSLoginCode(ctx *gin.Context) {
	type Req struct {
		Phone string `json:"phone"`
	}
	var req Req
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{
			"error": "系统错误0",
		})
		return
	}
	if req.Phone == "" {
		ctx.JSON(http.StatusOK, gin.H{
			"msg": "请输入手机号",
		})
		return
	}
	err := u.codeSvc.Send(ctx.Request.Context(), bizLogin, req.Phone)
	switch err {
	case nil:
		ctx.JSON(http.StatusOK, gin.H{
			"msg": "发送成功",
		})
	case service.ErrCodeSendTooMany:
		ctx.JSON(http.StatusOK, gin.H{
			"msg": "发送太频繁，请稍候再试",
		})
		return
	default:
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误",
		})
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
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "系统错误0"})
		return
	}
	//邮箱与密码匹配
	isEmail, err := u.emailRegexExp.MatchString(req.Email)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误1",
		})
		return
	}
	if !isEmail {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "邮箱格式错误",
		})
		return
	}
	if req.Password != req.ConfirmedPassword {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "两次输入的密码不一致",
		})
		return
	}
	isPassword, err := u.passwordRegexExp.MatchString(req.Password)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误2",
		})
		return
	}
	if !isPassword {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "密码必须包含数字、特殊字符，并且长度不能小于 8 位",
		})
		return
	}
	err = u.svc.SignUp(ctx.Request.Context(), domain.User{
		Email:    req.Email,
		Password: req.Password,
	})
	if errors.Is(err, service.ErrUserDuplicate) {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "邮箱已存在",
		})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误,注册失败",
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
		"msg": "注册成功",
	})
}

func (u *UserHandler) Profile(ctx *gin.Context) {
	val, ok := ctx.Get("user")
	if !ok {
		ctx.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	uc := val.(UserClaims)
	user, err := u.svc.Profile(ctx.Request.Context(), uc.Id)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误",
		})
		return
	}
	ctx.JSON(http.StatusOK, gin.H{
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
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误1",
		})
		return
	}
	user, err := u.svc.Login(ctx.Request.Context(),
		req.Email, req.Password,
	)
	if errors.Is(err, service.ErrInvalidUserOrPassword) {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "邮箱或密码不正确",
		})
		return
	}
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误2",
		})
		return
	}
	u.setJWTToken(ctx, user.Id)
	ctx.JSON(http.StatusOK, gin.H{
		"msg": "登录成功",
	})
}

func (u *UserHandler) setJWTToken(ctx *gin.Context, id int64) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		Id:        id,
		UserAgent: ctx.GetHeader("User-Agent"),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute * 30)),
		},
	})
	tokenStr, err := token.SignedString(JWTKey)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统异常",
		})
	}
	ctx.Header("x-jwt-token", tokenStr)
}
