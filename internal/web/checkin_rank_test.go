package web

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"user-center/internal/service"
	jwt2 "user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

type signInServiceStubForWeb struct {
	signInFn          func(ctx context.Context, userID int64) (service.SignInResult, error)
	getTodayStatusFn  func(ctx context.Context, userID int64) (bool, error)
	getMonthRecordsFn func(ctx context.Context, userID int64, year, month int) ([]int, error)
	getStreakFn       func(ctx context.Context, userID int64) (int, error)
}

func (s *signInServiceStubForWeb) SignIn(ctx context.Context, userID int64) (service.SignInResult, error) {
	if s.signInFn == nil {
		return service.SignInResult{}, nil
	}
	return s.signInFn(ctx, userID)
}

func (s *signInServiceStubForWeb) GetTodayStatus(ctx context.Context, userID int64) (bool, error) {
	if s.getTodayStatusFn == nil {
		return false, nil
	}
	return s.getTodayStatusFn(ctx, userID)
}

func (s *signInServiceStubForWeb) GetMonthRecords(ctx context.Context, userID int64, year, month int) ([]int, error) {
	if s.getMonthRecordsFn == nil {
		return nil, nil
	}
	return s.getMonthRecordsFn(ctx, userID, year, month)
}

func (s *signInServiceStubForWeb) GetStreak(ctx context.Context, userID int64) (int, error) {
	if s.getStreakFn == nil {
		return 0, nil
	}
	return s.getStreakFn(ctx, userID)
}

type rankServiceStubForWeb struct {
	getDailyTopNFn   func(ctx context.Context, limit int64) ([]service.RankUser, error)
	getMonthlyTopNFn func(ctx context.Context, limit int64) ([]service.RankUser, error)
	getDailyMeFn     func(ctx context.Context, userID int64) (service.RankUser, error)
	getMonthlyMeFn   func(ctx context.Context, userID int64) (service.RankUser, error)
}

func (s *rankServiceStubForWeb) GetDailyTopN(ctx context.Context, limit int64) ([]service.RankUser, error) {
	if s.getDailyTopNFn == nil {
		return nil, nil
	}
	return s.getDailyTopNFn(ctx, limit)
}

func (s *rankServiceStubForWeb) GetMonthlyTopN(ctx context.Context, limit int64) ([]service.RankUser, error) {
	if s.getMonthlyTopNFn == nil {
		return nil, nil
	}
	return s.getMonthlyTopNFn(ctx, limit)
}

func (s *rankServiceStubForWeb) GetDailyMe(ctx context.Context, userID int64) (service.RankUser, error) {
	if s.getDailyMeFn == nil {
		return service.RankUser{}, nil
	}
	return s.getDailyMeFn(ctx, userID)
}

func (s *rankServiceStubForWeb) GetMonthlyMe(ctx context.Context, userID int64) (service.RankUser, error) {
	if s.getMonthlyMeFn == nil {
		return service.RankUser{}, nil
	}
	return s.getMonthlyMeFn(ctx, userID)
}

func TestCheckInHandler_Today_AcceptsPointerClaimsFromMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt2.UserClaims{Id: 99})
	})
	h := NewCheckInHandler(&signInServiceStubForWeb{
		getTodayStatusFn: func(ctx context.Context, userID int64) (bool, error) {
			if userID != 99 {
				t.Fatalf("handler should read uid from *UserClaims, got %d", userID)
			}
			return true, nil
		},
	})
	h.RegisterRoutes(server)

	req := httptest.NewRequest(http.MethodGet, "/checkin/today", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("want status 200, got %d, body=%s", resp.Code, resp.Body.String())
	}
	var result Result
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result.Code != CodeSuccess || result.Msg != "查询成功" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRankHandler_DailyTopN_ParseLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name      string
		query     string
		wantLimit int64
	}{
		{name: "default limit", query: "/rank/daily", wantLimit: 10},
		{name: "valid limit", query: "/rank/daily?limit=20", wantLimit: 20},
		{name: "negative limit falls back to default", query: "/rank/daily?limit=-1", wantLimit: 10},
		{name: "zero limit falls back to default", query: "/rank/daily?limit=0", wantLimit: 10},
		{name: "too large falls back to default", query: "/rank/daily?limit=101", wantLimit: 10},
		{name: "non number falls back to default", query: "/rank/daily?limit=abc", wantLimit: 10},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := gin.New()
			h := NewRankHandler(&rankServiceStubForWeb{
				getDailyTopNFn: func(ctx context.Context, limit int64) ([]service.RankUser, error) {
					if limit != tc.wantLimit {
						t.Fatalf("want limit %d, got %d", tc.wantLimit, limit)
					}
					return []service.RankUser{}, nil
				},
			})
			h.RegisterRoutes(server)

			req := httptest.NewRequest(http.MethodGet, tc.query, nil)
			resp := httptest.NewRecorder()
			server.ServeHTTP(resp, req)

			if resp.Code != http.StatusOK {
				t.Fatalf("want status 200, got %d", resp.Code)
			}
		})
	}
}

func TestRankHandler_MyDaily_AcceptsPointerClaimsFromMiddleware(t *testing.T) {
	gin.SetMode(gin.TestMode)

	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt2.UserClaims{Id: 88})
	})
	h := NewRankHandler(&rankServiceStubForWeb{
		getDailyMeFn: func(ctx context.Context, userID int64) (service.RankUser, error) {
			if userID != 88 {
				t.Fatalf("handler should read uid from *UserClaims, got %d", userID)
			}
			return service.RankUser{UserID: userID, Rank: 1, Score: 5}, nil
		},
	})
	h.RegisterRoutes(server)

	req := httptest.NewRequest(http.MethodGet, "/rank/me/daily", nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("want status 200, got %d, body=%s", resp.Code, resp.Body.String())
	}
	var result Result
	if err := json.Unmarshal(resp.Body.Bytes(), &result); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if result.Code != CodeSuccess {
		t.Fatalf("unexpected result: %+v", result)
	}
}
