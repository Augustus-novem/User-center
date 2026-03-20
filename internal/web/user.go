package web

import (
	"errors"
	"net/http"

	"user-center/internal/domain"
	"user-center/internal/service"

	regexp "github.com/dlclark/regexp2"
	"github.com/gin-gonic/gin"
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
	ug.POST("signup", u.SignUp)
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
	if errors.As(err, &service.ErrUserDuplicateEmail) {
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

}

func (u *UserHandler) Edit(ctx *gin.Context) {

}
func (u *UserHandler) LogIn(ctx *gin.Context) {

}
