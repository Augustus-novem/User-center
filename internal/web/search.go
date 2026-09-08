package web

import (
	"errors"
	"strconv"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type SearchHandler struct {
	svc service.SearchService
}

func NewSearchHandler(svc service.SearchService) *SearchHandler {
	return &SearchHandler{svc: svc}
}

func (h *SearchHandler) RegisterRoutes(server *gin.Engine) {
	server.GET("/search/notes", h.SearchNotes)
}

func (h *SearchHandler) SearchNotes(ctx *gin.Context) {
	limit, _ := strconv.Atoi(ctx.Query("limit"))
	result, err := h.svc.SearchNotes(ctx.Request.Context(), ctx.Query("q"), limit)
	if errors.Is(err, service.ErrInvalidSearchQuery) {
		JSONBadRequest(ctx, "搜索关键词不合法")
		return
	}
	if err != nil {
		JSONInternalServerError(ctx, "搜索失败")
		return
	}
	JSONOK(ctx, "搜索成功", result)
}
