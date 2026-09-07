package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

type feedServiceStub struct {
	listFn   func(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error)
	fanoutFn func(ctx context.Context, noteID, authorID int64) error
}

func (s *feedServiceStub) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error) {
	if s.listFn == nil {
		return domain.NotePage{}, nil
	}
	return s.listFn(ctx, userID, cursor, limit)
}

func (s *feedServiceStub) FanoutPublished(ctx context.Context, noteID, authorID int64) error {
	if s.fanoutFn == nil {
		return nil
	}
	return s.fanoutFn(ctx, noteID, authorID)
}

func TestFeedHandler_Following(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt.UserClaims{Id: 5})
		ctx.Next()
	})
	NewFeedHandler(&feedServiceStub{
		listFn: func(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error) {
			if userID != 5 || cursor != "9" || limit != 2 {
				t.Fatalf("args user=%d cursor=%s limit=%d", userID, cursor, limit)
			}
			return domain.NotePage{Items: []domain.Note{{ID: 9, Title: "n"}}, HasMore: false}, nil
		},
	}).RegisterRoutes(server)

	req := httptest.NewRequest(http.MethodGet, "/feed/following?cursor=9&limit=2", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
	}
	var body Result
	_ = json.Unmarshal(resp.Body.Bytes(), &body)
	if body.Code != 0 {
		t.Fatalf("body=%+v", body)
	}
}
