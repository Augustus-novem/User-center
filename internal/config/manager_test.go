package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "app.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return path
}

func fullConfigYAML() string {
	return `
server:
  port: 8089
  mode: debug
db:
  dsn: root:root@tcp(localhost:13316)/webook
redis:
  addr: localhost:6379
jwt:
  access_token_key: access-in-yaml
  refresh_token_key: refresh-in-yaml
  access_token_ttl: 30m
  refresh_token_ttl: 168h
  idle_timeout: 168h
  absolute_timeout: 720h
wechat:
  app_id: wx-app-id
  app_key: wx-app-key
  redirect_url: http://localhost:8089/oauth2/wechat/callback
  state_cookie_name: jwt-state
  state_token_key: yaml-state-key
  state_token_ttl: 10m
  state_cookie_path: /oauth2/wechat/callback
cors:
  allow_origins: ["http://localhost:3000"]
ratelimit:
  enabled: true
  prefix: ip-limiter
  interval: 1m
  limit: 100
log:
  level: info
  encoding: console
  output_paths: ["stdout"]
  error_output_paths: ["stderr"]
  development: true
feature:
  enable_wechat_login: true
  enable_sms_login: false
  enable_debug_log: true
`
}

func TestNewManager_LoadsConfigAndEnvOverride(t *testing.T) {
	t.Setenv("DB_DSN", "env-user:env-pass@tcp(localhost:3306)/envdb")
	path := writeTempConfig(t, fullConfigYAML())

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}

	cfg := mgr.App()
	if cfg.DB.DSN != "env-user:env-pass@tcp(localhost:3306)/envdb" {
		t.Fatalf("expected env override for db.dsn, got %q", cfg.DB.DSN)
	}
	if cfg.JWT.AccessTokenTTL != 30*time.Minute {
		t.Fatalf("expected access ttl 30m, got %v", cfg.JWT.AccessTokenTTL)
	}
	if cfg.Wechat.StateTokenTTL != 10*time.Minute {
		t.Fatalf("expected state token ttl 10m, got %v", cfg.Wechat.StateTokenTTL)
	}
	if mgr.Dynamic().LogLevel != cfg.Log.Level {
		t.Fatalf("dynamic log level should mirror app config, got %q vs %q", mgr.Dynamic().LogLevel, cfg.Log.Level)
	}
	if mgr.Path() != path {
		t.Fatalf("unexpected manager path: %s", mgr.Path())
	}
}

func TestNewManager_AppliesDefaults(t *testing.T) {
	path := writeTempConfig(t, `
server:
  port: 8081
db:
  dsn: root:root@tcp(localhost:13316)/webook
redis:
  addr: localhost:6379
jwt:
  access_token_key: access-in-yaml
  refresh_token_key: refresh-in-yaml
wechat:
  app_id: wx-app-id
  app_key: wx-app-key
  state_token_key: yaml-state-key
`)

	mgr, err := NewManager(path)
	if err != nil {
		t.Fatalf("NewManager returned error: %v", err)
	}
	cfg := mgr.App()
	if cfg.Redis.DB != 1 {
		t.Fatalf("expected redis.db default=1, got %d", cfg.Redis.DB)
	}
	if cfg.JWT.AccessTokenTTL != 15*time.Minute {
		t.Fatalf("expected default access ttl 15m, got %v", cfg.JWT.AccessTokenTTL)
	}
	if cfg.Wechat.RedirectURL != "http://localhost:8081/oauth2/wechat/callback" {
		t.Fatalf("unexpected default redirect url: %s", cfg.Wechat.RedirectURL)
	}
	if cfg.Wechat.StateTokenTTL != 10*time.Minute {
		t.Fatalf("unexpected default state token ttl: %v", cfg.Wechat.StateTokenTTL)
	}
}

func TestValidate(t *testing.T) {
	base := AppConfig{
		Server: ServerConfig{Port: 8081},
		DB:     DBConfig{DSN: "root:root@tcp(localhost:13316)/webook"},
		Redis:  RedisConfig{Addr: "localhost:6379", DB: 1},
		JWT: JWTConfig{
			AccessTokenKey:  "access-key",
			RefreshTokenKey: "refresh-key",
			AccessTokenTTL:  15 * time.Minute,
			RefreshTokenTTL: 24 * time.Hour,
			IdleTimeout:     24 * time.Hour,
			AbsoluteTimeout: 7 * 24 * time.Hour,
		},
		Wechat: WechatConfig{
			AppID:           "wx-app-id",
			AppKey:          "wx-app-key",
			RedirectURL:     "http://localhost:8081/oauth2/wechat/callback",
			StateCookieName: "jwt-state",
			StateTokenKey:   "state-key",
			StateTokenTTL:   10 * time.Minute,
			StateCookiePath: "/oauth2/wechat/callback",
		},
		RateLimit: RateLimitConfig{Enabled: true, Limit: 100},
		Feature:   FeatureConfig{EnableWechatLogin: true},
	}

	tests := []struct {
		name    string
		mutate  func(cfg *AppConfig)
		wantErr string
	}{
		{
			name: "missing jwt access token key",
			mutate: func(cfg *AppConfig) {
				cfg.JWT.AccessTokenKey = ""
			},
			wantErr: "jwt.access_token_key 不能为空",
		},
		{
			name: "invalid ratelimit limit",
			mutate: func(cfg *AppConfig) {
				cfg.RateLimit.Limit = 0
			},
			wantErr: "ratelimit.limit 必须大于 0",
		},
		{
			name: "missing wechat state token key",
			mutate: func(cfg *AppConfig) {
				cfg.Wechat.StateTokenKey = ""
			},
			wantErr: "feature.enable_wechat_login=true 时，wechat.state_token_key 不能为空",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := base
			tc.mutate(&cfg)
			err := validate(cfg)
			if err == nil || err.Error() != tc.wantErr {
				t.Fatalf("want err %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestHolder_Update(t *testing.T) {
	cfg := AppConfig{
		Log: LogConfig{Level: "info"},
		Feature: FeatureConfig{
			EnableWechatLogin: true,
			EnableSMSLogin:    false,
		},
	}
	h := NewHolder(cfg)
	got := h.Get()
	if got.LogLevel != "info" || !got.Feature.EnableWechatLogin {
		t.Fatalf("unexpected initial dynamic config: %+v", got)
	}

	cfg.Log.Level = "debug"
	cfg.Feature.EnableSMSLogin = true
	h.Update(cfg)
	got = h.Get()
	if got.LogLevel != "debug" || !got.Feature.EnableSMSLogin {
		t.Fatalf("unexpected updated dynamic config: %+v", got)
	}
}

func TestWarnStaticChange(t *testing.T) {
	core, logs := observer.New(zapcore.WarnLevel)
	logger := zap.New(core)

	oldCfg := AppConfig{
		Server: ServerConfig{Port: 8081},
		DB:     DBConfig{DSN: "old"},
		Redis:  RedisConfig{Addr: "localhost:6379", DB: 1},
		JWT:    JWTConfig{AccessTokenKey: "a", RefreshTokenKey: "b"},
	}
	newCfg := oldCfg
	newCfg.DB.DSN = "new"
	newCfg.Redis.DB = 2

	warnStaticChange(logger, oldCfg, newCfg)
	entries := logs.All()
	if len(entries) != 1 {
		t.Fatalf("expected 1 warning log, got %d", len(entries))
	}
	ctx := entries[0].ContextMap()
	keys := ctx["keys"]
	joined := strings.TrimSpace(strings.ReplaceAll(strings.Trim(fmt.Sprint(keys), "[]"), " ", ","))
	if !strings.Contains(joined, "db") || !strings.Contains(joined, "redis") {
		t.Fatalf("expected warning keys to include db and redis, got %v", keys)
	}
}
