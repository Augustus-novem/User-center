package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"user-center/config"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/ioc"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func mustInitWebServer(t *testing.T) (*gin.Engine, *redis.Client) {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Config.Redis.Addr,
		Password: config.Config.Redis.Password,
		DB:       config.Config.Redis.DB,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}

	var (
		db     any
		server *gin.Engine
	)
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Skipf("mysql not available: %v", r)
			}
		}()
		gdb := ioc.InitDB()
		userDAO := dao.NewGORMUserDAO(gdb)
		userCache := cache.NewRedisUserCache(rdb)
		codeCache := cache.NewRedisCodeCache(rdb)
		userRepo := repository.NewCachedUserRepository(userDAO, userCache)
		codeRepo := repository.NewCachedCodeRepository(codeCache)
		userSvc := service.NewUserServiceImpl(userRepo)
		codeSvc := service.NewSMSCodeService(codeRepo, ioc.InitSmsMemoryService())
		hdl := web.NewUserHandler(userSvc, codeSvc)
		server = ioc.InitWebServer(ioc.GinMiddlwares(rdb), hdl)
		db = gdb
	}()
	_ = db
	return server, rdb
}

func TestUserHandler_SendSMSLoginCode(t *testing.T) {
	gin.SetMode(gin.TestMode)
	const sendSMSCodeURL = "/user/login_sms/code/send"
	server, rdb := mustInitWebServer(t)

	testCases := []struct {
		name       string
		before     func(t *testing.T)
		after      func(t *testing.T)
		phone      string
		wantCode   int
		wantResult web.Result
	}{
		{
			name:   "发送成功",
			before: func(t *testing.T) {},
			after: func(t *testing.T) {
				ctx := context.Background()
				key := "phone_code:login:15212345678"
				val, err := rdb.Get(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if len(val) != 6 {
					t.Fatalf("want 6-digit code, got %q", val)
				}
				ttl, err := rdb.TTL(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if ttl <= 9*time.Minute {
					t.Fatalf("want ttl > 9m, got %v", ttl)
				}
				if err = rdb.Del(ctx, key, key+":cnt").Err(); err != nil {
					t.Fatal(err)
				}
			},
			phone:      "15212345678",
			wantCode:   http.StatusOK,
			wantResult: web.Result{Code: web.CodeSuccess, Msg: "发送成功"},
		},
		{
			name:       "空的手机号码",
			before:     func(t *testing.T) {},
			after:      func(t *testing.T) {},
			phone:      "",
			wantCode:   http.StatusOK,
			wantResult: web.Result{Code: web.CodeBadRequest, Msg: "请输入手机号"},
		},
		{
			name: "发送太频繁",
			before: func(t *testing.T) {
				ctx := context.Background()
				key := "phone_code:login:15212345679"
				if err := rdb.Set(ctx, key, "123456", 9*time.Minute+30*time.Second).Err(); err != nil {
					t.Fatal(err)
				}
			},
			after: func(t *testing.T) {
				ctx := context.Background()
				key := "phone_code:login:15212345679"
				val, err := rdb.Get(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if val != "123456" {
					t.Fatalf("want preserved code 123456, got %s", val)
				}
				if err = rdb.Del(ctx, key, key+":cnt").Err(); err != nil {
					t.Fatal(err)
				}
			},
			phone:      "15212345679",
			wantCode:   http.StatusOK,
			wantResult: web.Result{Code: web.CodeBadRequest, Msg: "发送太频繁，请稍候再试"},
		},
		{
			name: "未知错误",
			before: func(t *testing.T) {
				ctx := context.Background()
				key := "phone_code:login:15212345670"
				if err := rdb.Set(ctx, key, "123456", 0).Err(); err != nil {
					t.Fatal(err)
				}
			},
			after: func(t *testing.T) {
				ctx := context.Background()
				key := "phone_code:login:15212345670"
				val, err := rdb.Get(ctx, key).Result()
				if err != nil {
					t.Fatal(err)
				}
				if val != "123456" {
					t.Fatalf("want preserved code 123456, got %s", val)
				}
				if err = rdb.Del(ctx, key, key+":cnt").Err(); err != nil {
					t.Fatal(err)
				}
			},
			phone:      "15212345670",
			wantCode:   http.StatusInternalServerError,
			wantResult: web.Result{Code: web.CodeInternalErr, Msg: "系统错误"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.before(t)
			body := fmt.Sprintf(`{"phone":"%s"}`, tc.phone)
			req, err := http.NewRequest(http.MethodPost, sendSMSCodeURL, bytes.NewBufferString(body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")

			recorder := httptest.NewRecorder()
			server.ServeHTTP(recorder, req)

			if recorder.Code != tc.wantCode {
				t.Fatalf("want status %d, got %d, body=%s", tc.wantCode, recorder.Code, recorder.Body.String())
			}
			var result web.Result
			if err = json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatalf("unmarshal response: %v, body=%s", err, recorder.Body.String())
			}
			if result.Code != tc.wantResult.Code || result.Msg != tc.wantResult.Msg {
				t.Fatalf("want result %+v, got %+v", tc.wantResult, result)
			}
			tc.after(t)
		})
	}
}
