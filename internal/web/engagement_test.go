package web

import (
	"bytes"
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

type engagementServiceStub struct {
	likeFn          func(ctx context.Context, userID, noteID int64) error
	unlikeFn        func(ctx context.Context, userID, noteID int64) error
	createCommentFn func(ctx context.Context, userID, noteID int64, content string) (domain.Comment, error)
	listCommentsFn  func(ctx context.Context, noteID int64, cursor string, limit int) (domain.CommentPage, error)
}

func (s *engagementServiceStub) Like(ctx context.Context, userID, noteID int64) error {
	if s.likeFn == nil {
		return nil
	}
	return s.likeFn(ctx, userID, noteID)
}

func (s *engagementServiceStub) Unlike(ctx context.Context, userID, noteID int64) error {
	if s.unlikeFn == nil {
		return nil
	}
	return s.unlikeFn(ctx, userID, noteID)
}

func (s *engagementServiceStub) CreateComment(ctx context.Context, userID, noteID int64, content string) (domain.Comment, error) {
	if s.createCommentFn == nil {
		return domain.Comment{ID: 1, UserID: userID, NoteID: noteID, Content: content}, nil
	}
	return s.createCommentFn(ctx, userID, noteID, content)
}

func (s *engagementServiceStub) ListComments(ctx context.Context, noteID int64, cursor string, limit int) (domain.CommentPage, error) {
	if s.listCommentsFn == nil {
		return domain.CommentPage{}, nil
	}
	return s.listCommentsFn(ctx, noteID, cursor, limit)
}

func TestEngagementHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt2.UserClaims{Id: 4})
		ctx.Next()
	})
	NewEngagementHandler(&engagementServiceStub{
		likeFn: func(ctx context.Context, userID, noteID int64) error {
			if userID != 4 || noteID != 8 {
				t.Fatalf("ids %d %d", userID, noteID)
			}
			return nil
		},
		createCommentFn: func(ctx context.Context, userID, noteID int64, content string) (domain.Comment, error) {
			if content == "" {
				return domain.Comment{}, service.ErrInvalidComment
			}
			return domain.Comment{ID: 3, UserID: userID, NoteID: noteID, Content: content}, nil
		},
	}).RegisterRoutes(server)

	likeReq := httptest.NewRequest(http.MethodPost, "/notes/8/like", nil)
	likeResp := httptest.NewRecorder()
	server.ServeHTTP(likeResp, likeReq)
	if likeResp.Code != http.StatusOK {
		t.Fatalf("like status=%d", likeResp.Code)
	}

	body, _ := json.Marshal(createCommentReq{Content: ""})
	cmtReq := httptest.NewRequest(http.MethodPost, "/notes/8/comments", bytes.NewReader(body))
	cmtReq.Header.Set("Content-Type", "application/json")
	cmtResp := httptest.NewRecorder()
	server.ServeHTTP(cmtResp, cmtReq)
	var parsed Result
	_ = json.Unmarshal(cmtResp.Body.Bytes(), &parsed)
	if parsed.Code != CodeBadRequest {
		t.Fatalf("empty comment should be rejected: %+v", parsed)
	}
}
