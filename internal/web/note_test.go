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

type noteServiceStub struct {
	publishFn      func(ctx context.Context, authorID int64, title, content string, imageURLs []string) (domain.Note, error)
	getFn          func(ctx context.Context, noteID int64) (domain.Note, error)
	deleteFn       func(ctx context.Context, operatorID, noteID int64) error
	listByAuthorFn func(ctx context.Context, authorID int64, cursor string, limit int) (domain.NotePage, error)
}

func (s *noteServiceStub) Publish(ctx context.Context, authorID int64, title, content string, imageURLs []string) (domain.Note, error) {
	if s.publishFn == nil {
		return domain.Note{ID: 1, AuthorID: authorID, Title: title, Content: content}, nil
	}
	return s.publishFn(ctx, authorID, title, content, imageURLs)
}

func (s *noteServiceStub) Get(ctx context.Context, noteID int64) (domain.Note, error) {
	if s.getFn == nil {
		return domain.Note{}, service.ErrNoteNotFound
	}
	return s.getFn(ctx, noteID)
}

func (s *noteServiceStub) Delete(ctx context.Context, operatorID, noteID int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, operatorID, noteID)
}

func (s *noteServiceStub) ListByAuthor(ctx context.Context, authorID int64, cursor string, limit int) (domain.NotePage, error) {
	if s.listByAuthorFn == nil {
		return domain.NotePage{}, nil
	}
	return s.listByAuthorFn(ctx, authorID, cursor, limit)
}

func TestNoteHandler_PublishAndDelete(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("publish keeps image urls", func(t *testing.T) {
		var gotURLs []string
		server := newNoteTestServer(4, &noteServiceStub{
			publishFn: func(ctx context.Context, authorID int64, title, content string, imageURLs []string) (domain.Note, error) {
				gotURLs = imageURLs
				return domain.Note{
					ID: 8, AuthorID: authorID, Title: title, Content: content,
					Images: []domain.NoteImage{{URL: imageURLs[0], SortOrder: 0}, {URL: imageURLs[1], SortOrder: 1}},
				}, nil
			},
		})
		body, _ := json.Marshal(publishNoteReq{Title: "t", Content: "c", ImageURLs: []string{"https://a", "https://b"}})
		req := httptest.NewRequest(http.MethodPost, "/notes", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if len(gotURLs) != 2 || gotURLs[0] != "https://a" {
			t.Fatalf("urls=%v", gotURLs)
		}
	})

	t.Run("non-author delete", func(t *testing.T) {
		server := newNoteTestServer(2, &noteServiceStub{
			deleteFn: func(ctx context.Context, operatorID, noteID int64) error {
				return service.ErrNoteForbidden
			},
		})
		req := httptest.NewRequest(http.MethodDelete, "/notes/9", nil)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)
		var body Result
		_ = json.Unmarshal(resp.Body.Bytes(), &body)
		if body.Code != CodeBadRequest || body.Msg != "无权操作该笔记" {
			t.Fatalf("unexpected body: %+v", body)
		}
	})
}

func newNoteTestServer(uid int64, svc service.NoteService) *gin.Engine {
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt2.UserClaims{Id: uid})
		ctx.Next()
	})
	NewNoteHandler(svc).RegisterRoutes(server)
	return server
}
