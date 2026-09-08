package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/service"
	jwt2 "user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

type notificationServiceWebStub struct {
	listFn     func(context.Context, int64, string, int) (domain.NotificationPage, error)
	markReadFn func(context.Context, int64, int64) error
}

func (*notificationServiceWebStub) CreateFollow(context.Context, string, int64, int64, int64) (bool, error) {
	return false, nil
}
func (*notificationServiceWebStub) CreateLike(context.Context, string, int64, int64, int64) (bool, error) {
	return false, nil
}
func (*notificationServiceWebStub) CreateComment(context.Context, string, int64, int64, int64, int64) (bool, error) {
	return false, nil
}
func (s *notificationServiceWebStub) List(ctx context.Context, receiverID int64, cursor string, limit int) (domain.NotificationPage, error) {
	return s.listFn(ctx, receiverID, cursor, limit)
}
func (s *notificationServiceWebStub) MarkRead(ctx context.Context, receiverID, notificationID int64) error {
	return s.markReadFn(ctx, receiverID, notificationID)
}

func TestNotificationHandler_ListUsesCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &notificationServiceWebStub{listFn: func(ctx context.Context, receiverID int64, cursor string, limit int) (domain.NotificationPage, error) {
		if receiverID != 7 || cursor != "100_9" || limit != 5 {
			t.Fatalf("receiver=%d cursor=%q limit=%d", receiverID, cursor, limit)
		}
		return domain.NotificationPage{Items: []domain.Notification{{
			ID: 8, ReceiverID: 7, ActorID: 2, Type: domain.NotificationTypeLike, BizID: 3, CreatedAt: 99,
		}}}, nil
	}}
	server := newNotificationTestServer(7, svc)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/notifications?cursor=100_9&limit=5", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var result Result
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || result.Code != CodeSuccess {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestNotificationHandler_MarkReadIsScopedToCurrentUser(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &notificationServiceWebStub{markReadFn: func(ctx context.Context, receiverID, notificationID int64) error {
		if receiverID != 7 || notificationID != 8 {
			t.Fatalf("receiver=%d notification=%d", receiverID, notificationID)
		}
		return nil
	}}
	server := newNotificationTestServer(7, svc)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notifications/8/read", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestNotificationHandler_CrossUserReadLooksNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &notificationServiceWebStub{markReadFn: func(context.Context, int64, int64) error {
		return service.ErrNotificationNotFound
	}}
	server := newNotificationTestServer(7, svc)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/notifications/8/read", nil))
	var result Result
	_ = json.Unmarshal(rec.Body.Bytes(), &result)
	if result.Code != CodeBadRequest || result.Msg != "通知不存在" {
		t.Fatalf("status=%d result=%+v", rec.Code, result)
	}
}

func newNotificationTestServer(userID int64, svc service.NotificationService) *gin.Engine {
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt2.UserClaims{Id: userID})
		ctx.Next()
	})
	NewNotificationHandler(svc).RegisterRoutes(server)
	return server
}
