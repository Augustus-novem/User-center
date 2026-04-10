package web

import (
	"strconv"
	"user-center/internal/pkg/biztime"
	"user-center/internal/service"
	jwt2 "user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

type CheckInHandler struct {
	svc service.SignInService
}

func NewCheckInHandler(svc service.SignInService) *CheckInHandler {
	return &CheckInHandler{
		svc: svc,
	}
}

func (h *CheckInHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/checkin")
	g.POST("", h.SignIn)
	g.GET("/today", h.Today)
	g.GET("/month", h.Month)
	g.GET("/streak", h.Streak)
}

func (h *CheckInHandler) SignIn(ctx *gin.Context) {
	uid, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	res, err := h.svc.SignIn(ctx.Request.Context(), uid)
	if err != nil {
		JSONInternalServerError(ctx, "签到失败")
		return
	}
	msg := "签到成功"
	if res.AlreadySigned {
		msg = "今日已签到"
	}
	JSONOK(ctx, msg, gin.H{
		"already_signed":  res.AlreadySigned,
		"continuous_days": res.ContinuousDays,
		"points":          res.Points,
	})
}

func (h *CheckInHandler) Today(ctx *gin.Context) {
	uid, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	signed, err := h.svc.GetTodayStatus(ctx.Request.Context(), uid)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{"signed": signed})
}

func (h *CheckInHandler) Month(ctx *gin.Context) {
	uid, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	year, month := biztime.CurrentYearMonth()
	if yearStr := ctx.Query("year"); yearStr != "" {
		parsed, err := strconv.Atoi(yearStr)
		if err != nil || parsed < 2000 || parsed > 3000 {
			JSONBadRequest(ctx, "year 参数错误")
			return
		}
		year = parsed
	}
	if monthStr := ctx.Query("month"); monthStr != "" {
		parsed, err := strconv.Atoi(monthStr)
		if err != nil || parsed < 1 || parsed > 12 {
			JSONBadRequest(ctx, "month 参数错误")
			return
		}
		month = parsed
	}
	days, err := h.svc.GetMonthRecords(ctx.Request.Context(), uid, year, month)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{
		"year":        year,
		"month":       month,
		"signed_days": days,
	})
}

func (h *CheckInHandler) Streak(ctx *gin.Context) {
	uid, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	streak, err := h.svc.GetStreak(ctx.Request.Context(), uid)
	if err != nil {
		JSONInternalServerError(ctx, "查询失败")
		return
	}
	JSONOK(ctx, "查询成功", gin.H{"continuous_days": streak})
}

func currentUserID(ctx *gin.Context) (int64, bool) {
	val, ok := ctx.Get("user")
	if !ok {
		return 0, false
	}
	uc, ok := val.(*jwt2.UserClaims)
	if !ok {
		return 0, false
	}
	return uc.Id, true
}
