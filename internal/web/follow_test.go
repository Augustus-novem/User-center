package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/service"
	jwt2 "user-center/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

type followServiceStub struct {
	followFn        func(ctx context.Context, followerID, followeeID int64) error
	unfollowFn      func(ctx context.Context, followerID, followeeID int64) error
	listFollowersFn func(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error)
	listFollowingFn func(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error)
}

func (s *followServiceStub) Follow(ctx context.Context, followerID, followeeID int64) error {
	if s.followFn == nil {
		return nil
	}
	return s.followFn(ctx, followerID, followeeID)
}

func (s *followServiceStub) Unfollow(ctx context.Context, followerID, followeeID int64) error {
	if s.unfollowFn == nil {
		return nil
	}
	return s.unfollowFn(ctx, followerID, followeeID)
}

func (s *followServiceStub) ListFollowers(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error) {
	if s.listFollowersFn == nil {
		return domain.FollowPage{}, nil
	}
	return s.listFollowersFn(ctx, userID, cursor, limit)
}

func (s *followServiceStub) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error) {
	if s.listFollowingFn == nil {
		return domain.FollowPage{}, nil
	}
	return s.listFollowingFn(ctx, userID, cursor, limit)
}

func TestFollowHandler_Follow(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("success", func(t *testing.T) {
		var gotFollower, gotFollowee int64
		server := newFollowTestServer(1, &followServiceStub{
			followFn: func(ctx context.Context, followerID, followeeID int64) error {
				gotFollower, gotFollowee = followerID, followeeID
				return nil
			},
		})
		resp := doFollowRequest(server, http.MethodPost, "/users/9/follow")
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		if gotFollower != 1 || gotFollowee != 9 {
			t.Fatalf("unexpected ids %d -> %d", gotFollower, gotFollowee)
		}
		assertFollowResult(t, resp.Body.Bytes(), 0, "关注成功")
	})

	t.Run("self follow", func(t *testing.T) {
		server := newFollowTestServer(9, &followServiceStub{
			followFn: func(ctx context.Context, followerID, followeeID int64) error {
				return service.ErrFollowSelf
			},
		})
		resp := doFollowRequest(server, http.MethodPost, "/users/9/follow")
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d", resp.Code)
		}
		assertFollowResult(t, resp.Body.Bytes(), CodeBadRequest, "不能关注自己")
	})

	t.Run("invalid id", func(t *testing.T) {
		server := newFollowTestServer(1, &followServiceStub{})
		resp := doFollowRequest(server, http.MethodPost, "/users/abc/follow")
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
	})
}

func TestFollowHandler_UnfollowAndList(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("unfollow success", func(t *testing.T) {
		server := newFollowTestServer(1, &followServiceStub{})
		resp := doFollowRequest(server, http.MethodDelete, "/users/2/follow")
		assertFollowResult(t, resp.Body.Bytes(), 0, "取消关注成功")
	})

	t.Run("followers page", func(t *testing.T) {
		server := newFollowTestServer(1, &followServiceStub{
			listFollowersFn: func(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error) {
				if userID != 2 || cursor != "100_3" || limit != 1 {
					t.Fatalf("unexpected args user=%d cursor=%s limit=%d", userID, cursor, limit)
				}
				return domain.FollowPage{
					Items:      []domain.FollowListItem{{UserID: 8, CreatedAt: 100}},
					NextCursor: "90_2",
					HasMore:    true,
				}, nil
			},
		})
		req := httptest.NewRequest(http.MethodGet, "/users/2/followers?cursor=100_3&limit=1", nil)
		resp := httptest.NewRecorder()
		server.ServeHTTP(resp, req)
		if resp.Code != http.StatusOK {
			t.Fatalf("status=%d body=%s", resp.Code, resp.Body.String())
		}
		var body Result
		if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if body.Code != 0 {
			t.Fatalf("unexpected body: %+v", body)
		}
	})

	t.Run("invalid cursor", func(t *testing.T) {
		server := newFollowTestServer(1, &followServiceStub{
			listFollowingFn: func(ctx context.Context, userID int64, cursor string, limit int) (domain.FollowPage, error) {
				return domain.FollowPage{}, service.ErrInvalidCursor
			},
		})
		resp := doFollowRequest(server, http.MethodGet, "/users/2/following?cursor=bad")
		assertFollowResult(t, resp.Body.Bytes(), CodeBadRequest, "游标无效")
	})

	t.Run("internal error", func(t *testing.T) {
		server := newFollowTestServer(1, &followServiceStub{
			followFn: func(ctx context.Context, followerID, followeeID int64) error {
				return errors.New("db down")
			},
		})
		resp := doFollowRequest(server, http.MethodPost, "/users/2/follow")
		if resp.Code != http.StatusInternalServerError {
			t.Fatalf("status=%d", resp.Code)
		}
	})
}

func newFollowTestServer(uid int64, svc service.FollowService) *gin.Engine {
	server := gin.New()
	server.Use(func(ctx *gin.Context) {
		ctx.Set("user", &jwt2.UserClaims{Id: uid})
		ctx.Next()
	})
	NewFollowHandler(svc).RegisterRoutes(server)
	return server
}

func doFollowRequest(server *gin.Engine, method, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	resp := httptest.NewRecorder()
	server.ServeHTTP(resp, req)
	return resp
}

func assertFollowResult(t *testing.T, raw []byte, code int, msg string) {
	t.Helper()
	var body Result
	if err := json.Unmarshal(raw, &body); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, raw)
	}
	if body.Code != code || body.Msg != msg {
		t.Fatalf("want code=%d msg=%q, got %+v", code, msg, body)
	}
}
