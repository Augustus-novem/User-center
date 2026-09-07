package web

import (
	"errors"
	"strconv"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type HotRankHandler struct {
	svc service.HotRankService
}

func NewHotRankHandler(svc service.HotRankService) *HotRankHandler {
	return &HotRankHandler{svc: svc}
}

func (h *HotRankHandler) RegisterRoutes(server *gin.Engine) {
	server.GET("/rank/hot", h.List)
}

func (h *HotRankHandler) List(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	page, err := h.svc.List(ctx.Request.Context(), ctx.Query("cursor"), limit)
	if errors.Is(err, service.ErrInvalidHotRankCursor) {
		JSONBadRequest(ctx, "游标无效或已过期")
		return
	}
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", page)
}
