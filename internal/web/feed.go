package web

import (
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type FeedHandler struct {
	svc service.FeedService
}

func NewFeedHandler(svc service.FeedService) *FeedHandler {
	return &FeedHandler{svc: svc}
}

func (h *FeedHandler) RegisterRoutes(server *gin.Engine) {
	server.GET("/feed/following", h.Following)
}

func (h *FeedHandler) Following(ctx *gin.Context) {
	userID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	limit, ok := parseFollowLimit(ctx)
	if !ok {
		return
	}
	page, err := h.svc.ListFollowing(ctx.Request.Context(), userID, ctx.Query("cursor"), limit)
	if err != nil {
		writeNoteError(ctx, err)
		return
	}
	JSONOK(ctx, "查询成功", toNotePageVO(page))
}
