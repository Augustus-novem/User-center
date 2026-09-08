package web

import (
	"errors"
	"strconv"
	"user-center/internal/domain"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type NotificationHandler struct {
	service service.NotificationService
}

func NewNotificationHandler(svc service.NotificationService) *NotificationHandler {
	return &NotificationHandler{service: svc}
}

func (h *NotificationHandler) RegisterRoutes(server *gin.Engine) {
	server.GET("/notifications", h.List)
	server.POST("/notifications/:id/read", h.MarkRead)
}

type notificationVO struct {
	ID        int64  `json:"id"`
	ActorID   int64  `json:"actor_id"`
	Type      string `json:"type"`
	BizID     int64  `json:"biz_id"`
	IsRead    bool   `json:"is_read"`
	CreatedAt int64  `json:"created_at"`
}

type notificationPageVO struct {
	Items      []notificationVO `json:"items"`
	NextCursor string           `json:"next_cursor,omitempty"`
	HasMore    bool             `json:"has_more"`
}

func (h *NotificationHandler) List(ctx *gin.Context) {
	receiverID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	limit, err := parseNotificationLimit(ctx)
	if err != nil {
		JSONBadRequest(ctx, "limit 参数错误")
		return
	}
	page, err := h.service.List(ctx.Request.Context(), receiverID, ctx.Query("cursor"), limit)
	if err != nil {
		writeNotificationError(ctx, err)
		return
	}
	JSONOK(ctx, "查询成功", toNotificationPageVO(page))
}

func (h *NotificationHandler) MarkRead(ctx *gin.Context) {
	receiverID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	notificationID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || notificationID <= 0 {
		JSONBadRequest(ctx, "通知 ID 无效")
		return
	}
	if err = h.service.MarkRead(ctx.Request.Context(), receiverID, notificationID); err != nil {
		writeNotificationError(ctx, err)
		return
	}
	JSONOK(ctx, "已标记为已读", gin.H{"is_read": true})
}

func parseNotificationLimit(ctx *gin.Context) (int, error) {
	raw := ctx.Query("limit")
	if raw == "" {
		return 0, nil
	}
	return strconv.Atoi(raw)
}

func writeNotificationError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrInvalidNotification),
		errors.Is(err, service.ErrInvalidNotificationCursor):
		JSONBadRequest(ctx, err.Error())
	case errors.Is(err, service.ErrNotificationNotFound):
		JSONBizError(ctx, "通知不存在")
	default:
		JSONInternalServerError(ctx, "系统错误")
	}
}

func toNotificationPageVO(page domain.NotificationPage) notificationPageVO {
	items := make([]notificationVO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, notificationVO{
			ID: item.ID, ActorID: item.ActorID, Type: item.Type,
			BizID: item.BizID, IsRead: item.IsRead, CreatedAt: item.CreatedAt,
		})
	}
	return notificationPageVO{Items: items, NextCursor: page.NextCursor, HasMore: page.HasMore}
}
