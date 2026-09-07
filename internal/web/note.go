package web

import (
	"errors"
	"strconv"
	"user-center/internal/domain"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type NoteHandler struct {
	svc service.NoteService
}

func NewNoteHandler(svc service.NoteService) *NoteHandler {
	return &NoteHandler{svc: svc}
}

func (h *NoteHandler) RegisterRoutes(server *gin.Engine) {
	notes := server.Group("/notes")
	notes.POST("", h.Publish)
	notes.GET("/:id", h.Get)
	notes.DELETE("/:id", h.Delete)
	server.GET("/users/:id/notes", h.ListByAuthor)
}

type publishNoteReq struct {
	Title     string   `json:"title"`
	Content   string   `json:"content"`
	ImageURLs []string `json:"image_urls"`
}

type noteImageVO struct {
	URL       string `json:"url"`
	SortOrder int    `json:"sort_order"`
}

type noteVO struct {
	ID        int64         `json:"id"`
	AuthorID  int64         `json:"author_id"`
	Title     string        `json:"title"`
	Content   string        `json:"content"`
	Status    string        `json:"status"`
	Images    []noteImageVO `json:"images"`
	CreatedAt int64         `json:"created_at"`
	UpdatedAt int64         `json:"updated_at"`
}

type notePageVO struct {
	Items      []noteVO `json:"items"`
	NextCursor string   `json:"next_cursor,omitempty"`
	HasMore    bool     `json:"has_more"`
}

func (h *NoteHandler) Publish(ctx *gin.Context) {
	authorID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	var req publishNoteReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	note, err := h.svc.Publish(ctx.Request.Context(), authorID, req.Title, req.Content, req.ImageURLs)
	if err != nil {
		writeNoteError(ctx, err)
		return
	}
	JSONOK(ctx, "发布成功", toNoteVO(note))
}

func (h *NoteHandler) Get(ctx *gin.Context) {
	if _, ok := currentUserID(ctx); !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	noteID, ok := parsePathID(ctx, "id", "笔记 ID 无效")
	if !ok {
		return
	}
	note, err := h.svc.Get(ctx.Request.Context(), noteID)
	if err != nil {
		writeNoteError(ctx, err)
		return
	}
	JSONOK(ctx, "查询成功", toNoteVO(note))
}

func (h *NoteHandler) Delete(ctx *gin.Context) {
	operatorID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	noteID, ok := parsePathID(ctx, "id", "笔记 ID 无效")
	if !ok {
		return
	}
	if err := h.svc.Delete(ctx.Request.Context(), operatorID, noteID); err != nil {
		writeNoteError(ctx, err)
		return
	}
	JSONOK(ctx, "删除成功", nil)
}

func (h *NoteHandler) ListByAuthor(ctx *gin.Context) {
	if _, ok := currentUserID(ctx); !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	authorID, ok := parsePathID(ctx, "id", "用户 ID 无效")
	if !ok {
		return
	}
	limit, ok := parseFollowLimit(ctx)
	if !ok {
		return
	}
	page, err := h.svc.ListByAuthor(ctx.Request.Context(), authorID, ctx.Query("cursor"), limit)
	if err != nil {
		writeNoteError(ctx, err)
		return
	}
	JSONOK(ctx, "查询成功", toNotePageVO(page))
}

func parsePathID(ctx *gin.Context, name, invalidMsg string) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param(name), 10, 64)
	if err != nil || id <= 0 {
		JSONBadRequest(ctx, invalidMsg)
		return 0, false
	}
	return id, true
}

func writeNoteError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidNote),
		errors.Is(err, service.ErrInvalidNoteID),
		errors.Is(err, service.ErrInvalidNoteCursor):
		JSONBizError(ctx, err.Error())
	case errors.Is(err, service.ErrNoteNotFound):
		JSONBizError(ctx, "笔记不存在")
	case errors.Is(err, service.ErrNoteForbidden):
		JSONBizError(ctx, "无权操作该笔记")
	default:
		JSONInternalServerError(ctx, "系统错误")
	}
}

func toNoteVO(note domain.Note) noteVO {
	images := make([]noteImageVO, 0, len(note.Images))
	for _, img := range note.Images {
		images = append(images, noteImageVO{URL: img.URL, SortOrder: img.SortOrder})
	}
	return noteVO{
		ID:        note.ID,
		AuthorID:  note.AuthorID,
		Title:     note.Title,
		Content:   note.Content,
		Status:    note.Status,
		Images:    images,
		CreatedAt: note.CreatedAt,
		UpdatedAt: note.UpdatedAt,
	}
}

func toNotePageVO(page domain.NotePage) notePageVO {
	items := make([]noteVO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, toNoteVO(item))
	}
	return notePageVO{Items: items, NextCursor: page.NextCursor, HasMore: page.HasMore}
}
