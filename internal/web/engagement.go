package web

import (
	"errors"
	"user-center/internal/domain"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type EngagementHandler struct {
	svc service.EngagementService
}

func NewEngagementHandler(svc service.EngagementService) *EngagementHandler {
	return &EngagementHandler{svc: svc}
}

func (h *EngagementHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/notes")
	g.POST("/:id/like", h.Like)
	g.DELETE("/:id/like", h.Unlike)
	g.POST("/:id/comments", h.CreateComment)
	g.GET("/:id/comments", h.ListComments)
}

type createCommentReq struct {
	Content string `json:"content"`
}

type commentVO struct {
	ID        int64  `json:"id"`
	NoteID    int64  `json:"note_id"`
	UserID    int64  `json:"user_id"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
}

type commentPageVO struct {
	Items      []commentVO `json:"items"`
	NextCursor string      `json:"next_cursor,omitempty"`
	HasMore    bool        `json:"has_more"`
}

func (h *EngagementHandler) Like(ctx *gin.Context) {
	userID, noteID, ok := h.currentAndNoteID(ctx)
	if !ok {
		return
	}
	if err := h.svc.Like(ctx.Request.Context(), userID, noteID); err != nil {
		writeEngagementError(ctx, err)
		return
	}
	JSONOK(ctx, "点赞成功", gin.H{"liked": true})
}

func (h *EngagementHandler) Unlike(ctx *gin.Context) {
	userID, noteID, ok := h.currentAndNoteID(ctx)
	if !ok {
		return
	}
	if err := h.svc.Unlike(ctx.Request.Context(), userID, noteID); err != nil {
		writeEngagementError(ctx, err)
		return
	}
	JSONOK(ctx, "取消点赞成功", gin.H{"liked": false})
}

func (h *EngagementHandler) CreateComment(ctx *gin.Context) {
	userID, noteID, ok := h.currentAndNoteID(ctx)
	if !ok {
		return
	}
	var req createCommentReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	comment, err := h.svc.CreateComment(ctx.Request.Context(), userID, noteID, req.Content)
	if err != nil {
		writeEngagementError(ctx, err)
		return
	}
	JSONOK(ctx, "评论成功", toCommentVO(comment))
}

func (h *EngagementHandler) ListComments(ctx *gin.Context) {
	if _, ok := currentUserID(ctx); !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	noteID, ok := parsePathID(ctx, "id", "笔记 ID 无效")
	if !ok {
		return
	}
	limit, ok := parseFollowLimit(ctx)
	if !ok {
		return
	}
	page, err := h.svc.ListComments(ctx.Request.Context(), noteID, ctx.Query("cursor"), limit)
	if err != nil {
		writeEngagementError(ctx, err)
		return
	}
	JSONOK(ctx, "查询成功", toCommentPageVO(page))
}

func (h *EngagementHandler) currentAndNoteID(ctx *gin.Context) (int64, int64, bool) {
	userID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return 0, 0, false
	}
	noteID, ok := parsePathID(ctx, "id", "笔记 ID 无效")
	if !ok {
		return 0, 0, false
	}
	return userID, noteID, true
}

func writeEngagementError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidComment),
		errors.Is(err, service.ErrInvalidNoteID),
		errors.Is(err, service.ErrInvalidNoteCursor):
		JSONBizError(ctx, err.Error())
	case errors.Is(err, service.ErrNoteNotFound):
		JSONBizError(ctx, "笔记不存在")
	default:
		JSONInternalServerError(ctx, "系统错误")
	}
}

func toCommentVO(c domain.Comment) commentVO {
	return commentVO{
		ID:        c.ID,
		NoteID:    c.NoteID,
		UserID:    c.UserID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
	}
}

func toCommentPageVO(page domain.CommentPage) commentPageVO {
	items := make([]commentVO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toCommentVO(item))
	}
	return commentPageVO{Items: items, NextCursor: page.NextCursor, HasMore: page.HasMore}
}
