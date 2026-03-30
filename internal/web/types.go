package web

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type Result struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data any    `json:"data,omitempty"`
}

const (
	CodeSuccess      = 0
	CodeBadRequest   = 400
	CodeUnauthorized = 401
	CodeInternalErr  = 500
)

func JSONOK(ctx *gin.Context, msg string, data any) {
	ctx.JSON(http.StatusOK, Result{
		Code: CodeSuccess,
		Msg:  msg,
		Data: data,
	})
}

func JSONBadRequest(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusBadRequest, Result{
		Code: CodeBadRequest,
		Msg:  msg,
	})
}

func JSONUnauthorized(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusUnauthorized, Result{
		Code: CodeUnauthorized,
		Msg:  msg,
	})
}

func JSONInternalServerError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusInternalServerError, Result{
		Code: CodeInternalErr,
		Msg:  msg,
	})
}

func JSONBizError(ctx *gin.Context, msg string) {
	ctx.JSON(http.StatusOK, Result{
		Code: CodeBadRequest,
		Msg:  msg,
	})
}

type handler interface {
	RegisterRoutes(s *gin.Engine)
}
