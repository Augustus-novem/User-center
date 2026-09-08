package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type searchServiceStub struct {
	searchFn func(context.Context, string, int) (service.NoteSearchResult, error)
}

func (s *searchServiceStub) SearchNotes(ctx context.Context, query string, limit int) (service.NoteSearchResult, error) {
	return s.searchFn(ctx, query, limit)
}

func TestSearchHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	NewSearchHandler(&searchServiceStub{searchFn: func(ctx context.Context, query string, limit int) (service.NoteSearchResult, error) {
		if query != "Go" || limit != 7 {
			t.Fatalf("query=%q limit=%d", query, limit)
		}
		return service.NoteSearchResult{Degraded: true}, nil
	}}).RegisterRoutes(server)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search/notes?q=Go&limit=7", nil))
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), `"degraded":true`) {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestSearchHandlerRejectsEmptyQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	NewSearchHandler(&searchServiceStub{searchFn: func(context.Context, string, int) (service.NoteSearchResult, error) {
		return service.NoteSearchResult{}, service.ErrInvalidSearchQuery
	}}).RegisterRoutes(server)
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/search/notes", nil))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body.String())
	}
}
