package web

import (
	"context"
	"errors"
	"strconv"
	"user-center/internal/domain"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type FollowHandler struct {
	svc service.FollowService
}

func NewFollowHandler(svc service.FollowService) *FollowHandler {
	return &FollowHandler{svc: svc}
}

func (h *FollowHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/users")
	g.POST("/:id/follow", h.Follow)
	g.DELETE("/:id/follow", h.Unfollow)
	g.GET("/:id/followers", h.Followers)
	g.GET("/:id/following", h.Following)
}

type followListItemVO struct {
	UserID    int64 `json:"user_id"`
	CreatedAt int64 `json:"created_at"`
}

type followPageVO struct {
	Items      []followListItemVO `json:"items"`
	NextCursor string             `json:"next_cursor,omitempty"`
	HasMore    bool               `json:"has_more"`
}

func (h *FollowHandler) Follow(ctx *gin.Context) {
	followerID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	followeeID, ok := parsePathUserID(ctx)
	if !ok {
		return
	}
	if err := h.svc.Follow(ctx.Request.Context(), followerID, followeeID); err != nil {
		writeFollowError(ctx, err)
		return
	}
	JSONOK(ctx, "关注成功", gin.H{"following": true})
}

func (h *FollowHandler) Unfollow(ctx *gin.Context) {
	followerID, ok := currentUserID(ctx)
	if !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	followeeID, ok := parsePathUserID(ctx)
	if !ok {
		return
	}
	if err := h.svc.Unfollow(ctx.Request.Context(), followerID, followeeID); err != nil {
		writeFollowError(ctx, err)
		return
	}
	JSONOK(ctx, "取消关注成功", gin.H{"following": false})
}

func (h *FollowHandler) Followers(ctx *gin.Context) {
	h.listRelations(ctx, h.svc.ListFollowers)
}

func (h *FollowHandler) Following(ctx *gin.Context) {
	h.listRelations(ctx, h.svc.ListFollowing)
}

func (h *FollowHandler) listRelations(ctx *gin.Context, listFn func(context.Context, int64, string, int) (domain.FollowPage, error)) {
	if _, ok := currentUserID(ctx); !ok {
		JSONUnauthorized(ctx, "请先登录")
		return
	}
	userID, ok := parsePathUserID(ctx)
	if !ok {
		return
	}
	limit, ok := parseFollowLimit(ctx)
	if !ok {
		return
	}
	page, err := listFn(ctx.Request.Context(), userID, ctx.Query("cursor"), limit)
	if err != nil {
		writeFollowError(ctx, err)
		return
	}
	JSONOK(ctx, "查询成功", toFollowPageVO(page))
}

func parsePathUserID(ctx *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		JSONBadRequest(ctx, "用户 ID 无效")
		return 0, false
	}
	return id, true
}

func parseFollowLimit(ctx *gin.Context) (int, bool) {
	raw := ctx.Query("limit")
	if raw == "" {
		return 0, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		JSONBadRequest(ctx, "limit 参数错误")
		return 0, false
	}
	return limit, true
}

func writeFollowError(ctx *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrFollowSelf),
		errors.Is(err, service.ErrFolloweeNotFound),
		errors.Is(err, service.ErrInvalidFollowID),
		errors.Is(err, service.ErrInvalidCursor):
		JSONBizError(ctx, err.Error())
	default:
		JSONInternalServerError(ctx, "系统错误")
	}
}

func toFollowPageVO(page domain.FollowPage) followPageVO {
	items := make([]followListItemVO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, followListItemVO{
			UserID:    item.UserID,
			CreatedAt: item.CreatedAt,
		})
	}
	return followPageVO{
		Items:      items,
		NextCursor: page.NextCursor,
		HasMore:    page.HasMore,
	}
}
