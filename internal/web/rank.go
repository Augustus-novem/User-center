package web

import (
	"strconv"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type RankHandler struct {
	svc service.RankService
}

func NewRankHandler(svc service.RankService) *RankHandler {
	return &RankHandler{svc: svc}
}

func (h *RankHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/rank")
	g.GET("/daily", h.DailyTopN)
	g.GET("/monthly", h.MonthlyTopN)
	g.GET("/me/daily", h.MyDaily)
	g.GET("/me/monthly", h.MyMonthly)
}

func (h *RankHandler) DailyTopN(ctx *gin.Context) {
	limit := h.parseLimit(ctx)
	items, err := h.svc.GetDailyTopN(ctx.Request.Context(), limit)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{"items": items})
}

func (h *RankHandler) MonthlyTopN(ctx *gin.Context) {
	limit := h.parseLimit(ctx)
	items, err := h.svc.GetMonthlyTopN(ctx.Request.Context(), limit)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{"items": items})
}

func (h *RankHandler) parseLimit(ctx *gin.Context) int64 {
	limit := int64(10)
	if limitStr := ctx.Query("limit"); limitStr != "" {
		parsed, err := strconv.ParseInt(limitStr, 10, 64)
		if err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	return limit
}

func (h *RankHandler) MyDaily(ctx *gin.Context) {
	uid, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	item, err := h.svc.GetDailyMe(ctx.Request.Context(), uid)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{"item": item})
}

func (h *RankHandler) MyMonthly(ctx *gin.Context) {
	uid, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	item, err := h.svc.GetMonthlyMe(ctx.Request.Context(), uid)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{"item": item})
}
