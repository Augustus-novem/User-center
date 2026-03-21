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

const emailRegexPattern = "^\\w+([-+.]\\w+)*@\\w+([-.]\\w+)*\\.\\w+([-.]\\w+)*$"
const passwordRegexPattern = `^(?=.*[A-Za-z])(?=.*\d)(?=.*[$@$!%*#?&])[A-Za-z\d$@$!%*#?&]{8,}$`

type UserHandler struct {
	svc              *service.UserService
	emailRegexExp    *regexp.Regexp
	passwordRegexExp *regexp.Regexp
}

func NewUserHandler(svc *service.UserService) *UserHandler {
	return &UserHandler{
		svc:              svc,
		emailRegexExp:    regexp.MustCompile(emailRegexPattern, regexp.None),
		passwordRegexExp: regexp.MustCompile(passwordRegexPattern, regexp.None),
	}
}

func (u *UserHandler) Register(server *gin.Engine) {
	ug := server.Group("/user")
	ug.GET("/profile", u.Profile)
	ug.POST("/signup", u.SignUp)
	ug.POST("/login", u.LogIn)
	ug.POST("/edit", u.Edit)
}

func (u *UserHandler) SignUp(ctx *gin.Context) {
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
	if errors.Is(err, service.ErrUserDuplicateEmail) {
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
		"email": user.Email,
	})
}

func (u *UserHandler) Edit(ctx *gin.Context) {

}

func (u *UserHandler) LogIn(ctx *gin.Context) {
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
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, UserClaims{
		Id: user.Id,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		},
	})
	tokenStr, err := token.SignedString(JWTKey)
	if err != nil {
		ctx.JSON(http.StatusOK, gin.H{
			"error": "系统错误3",
		})
		return
	}
	ctx.Header("x-jwt-token", tokenStr)
	ctx.JSON(http.StatusOK, gin.H{
		"msg": "登录成功",
	})
}
