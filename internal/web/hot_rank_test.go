package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type hotRankServiceStub struct {
	listFn func(ctx context.Context, cursor string, limit int) (service.HotRankPage, error)
}

func (*hotRankServiceStub) Record(context.Context, string, string, int64, int64) (bool, error) {
	return false, nil
}

func (s *hotRankServiceStub) List(ctx context.Context, cursor string, limit int) (service.HotRankPage, error) {
	return s.listFn(ctx, cursor, limit)
}

func TestHotRankHandler_List(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := &hotRankServiceStub{listFn: func(ctx context.Context, cursor string, limit int) (service.HotRankPage, error) {
		if cursor != "100_2" || limit != 5 {
			t.Fatalf("cursor=%q limit=%d", cursor, limit)
		}
		return service.HotRankPage{Items: []service.HotRankItem{{NoteID: 7, Score: 12, Rank: 3}}, SnapshotMinute: 100}, nil
	}}
	server := gin.New()
	NewHotRankHandler(svc).RegisterRoutes(server)
	req := httptest.NewRequest(http.MethodGet, "/rank/hot?cursor=100_2&limit=5", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestHotRankHandler_InvalidCursor(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := gin.New()
	NewHotRankHandler(&hotRankServiceStub{listFn: func(context.Context, string, int) (service.HotRankPage, error) {
		return service.HotRankPage{}, service.ErrInvalidHotRankCursor
	}}).RegisterRoutes(server)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/rank/hot?cursor=bad", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
