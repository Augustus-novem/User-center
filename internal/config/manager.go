package config

import (
	"flag"
	"fmt"
	"reflect"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"sync"
)

type Manager struct {
	mu     sync.RWMutex
	path   string
	app    AppConfig
	v      *viper.Viper
	holder *Holder
}

func NewManagerFromFlags() (*Manager, error) {
	configPath := flag.String("config", "config/dev.yaml", "config file path")
	if !flag.Parsed() {
		flag.Parse()
	}
	return NewManager(*configPath)
}

func NewManager(configPath string) (*Manager, error) {
	v := viper.New()
	setDefaults(v)
	bindEnvs(v)
	v.SetConfigFile(configPath)

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("read config file %s: %w", configPath, err)
	}

	var appcfg AppConfig
	if err := v.Unmarshal(&appcfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if err := validate(appcfg); err != nil {
		return nil, err
	}

	return &Manager{
		path:   configPath,
		v:      v,
		app:    appcfg,
		holder: NewHolder(appcfg),
	}, nil
}

func (m *Manager) Path() string {
	return m.path
}

func (m *Manager) App() AppConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.app
}

func (m *Manager) Dynamic() DynamicConfig {
	return m.holder.Get()
}

func (m *Manager) StartWatch(logger *zap.Logger, atomicLevel *zap.AtomicLevel) {
	m.v.OnConfigChange(func(e fsnotify.Event) {
		var latest AppConfig
		if err := m.v.Unmarshal(&latest); err != nil {
			logger.Error("配置文件变更后解析失败", zap.String("file", e.Name), zap.Error(err))
			return
		}
		if err := validate(latest); err != nil {
			logger.Error("配置文件变更后校验失败", zap.String("file", e.Name), zap.Error(err))
			return
		}

		m.mu.Lock()
		old := m.app
		m.app.Log.Level = latest.Log.Level
		m.app.Feature = latest.Feature
		m.mu.Unlock()

		if old.Log.Level != latest.Log.Level {
			lvl, err := zapcore.ParseLevel(latest.Log.Level)
			if err != nil {
				logger.Error("新的 log.level 非法，忽略本次热更新", zap.String("level", latest.Log.Level), zap.Error(err))
			} else {
				atomicLevel.SetLevel(lvl)
			}
		}
		m.holder.Update(latest)

		logger.Info("已热更新动态配置",
			zap.String("file", e.Name),
			zap.String("log_level", latest.Log.Level),
			zap.Bool("feature.enable_wechat_login", latest.Feature.EnableWechatLogin),
			zap.Bool("feature.enable_sms_login", latest.Feature.EnableSMSLogin),
			zap.Bool("feature.enable_debug_log", latest.Feature.EnableDebugLog),
		)
		warnStaticChange(logger, old, latest)
	})
	m.v.WatchConfig()
}

func warnStaticChange(logger *zap.Logger, oldCfg, newCfg AppConfig) {
	warnings := make([]string, 0, 9)
	if !reflect.DeepEqual(oldCfg.Server, newCfg.Server) {
		warnings = append(warnings, "server")
	}
	if !reflect.DeepEqual(oldCfg.DB, newCfg.DB) {
		warnings = append(warnings, "db")
	}
	if !reflect.DeepEqual(oldCfg.Redis, newCfg.Redis) {
		warnings = append(warnings, "redis")
	}
	if !reflect.DeepEqual(oldCfg.Kafka, newCfg.Kafka) {
		warnings = append(warnings, "kafka")
	}
	if !reflect.DeepEqual(oldCfg.JWT, newCfg.JWT) {
		warnings = append(warnings, "jwt")
	}
	if !reflect.DeepEqual(oldCfg.Wechat, newCfg.Wechat) {
		warnings = append(warnings, "wechat")
	}
	if !reflect.DeepEqual(oldCfg.CORS, newCfg.CORS) {
		warnings = append(warnings, "cors")
	}
	if !reflect.DeepEqual(oldCfg.RateLimit, newCfg.RateLimit) {
		warnings = append(warnings, "ratelimit")
	}
	if !reflect.DeepEqual(oldCfg.RAG, newCfg.RAG) {
		warnings = append(warnings, "rag")
	}
	if len(warnings) > 0 {
		logger.Warn("检测到静态配置变更；当前进程不会热更新这些模块，需重启后生效", zap.Strings("keys", warnings))
	}
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("server.name", "user-center")
	v.SetDefault("server.port", 8081)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("redis.db", 1)
	v.SetDefault("kafka.enabled", false)
	v.SetDefault("kafka.brokers", []string{"localhost:9092"})
	v.SetDefault("kafka.client_id", "user-center")
	v.SetDefault("kafka.consumer_group", "user-center-worker")
	v.SetDefault("jwt.access_token_ttl", "15m")
	v.SetDefault("jwt.refresh_token_ttl", "168h")
	v.SetDefault("jwt.idle_timeout", "168h")
	v.SetDefault("jwt.absolute_timeout", "720h")
	v.SetDefault("wechat.redirect_url", "http://localhost:8081/oauth2/wechat/callback")
	v.SetDefault("wechat.state_cookie_name", "jwt-state")
	v.SetDefault("wechat.state_token_ttl", "10m")
	v.SetDefault("wechat.state_cookie_path", "/oauth2/wechat/callback")
	v.SetDefault("cors.allow_credentials", true)
	v.SetDefault("cors.allow_methods", []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"})
	v.SetDefault("cors.allow_headers", []string{"Content-Type", "Authorization", "X-Refresh-Token"})
	v.SetDefault("cors.expose_headers", []string{"x-jwt-token", "x-refresh-token"})
	v.SetDefault("cors.max_age", "12h")
	v.SetDefault("ratelimit.enabled", true)
	v.SetDefault("ratelimit.prefix", "ip-limiter")
	v.SetDefault("ratelimit.interval", "1m")
	v.SetDefault("ratelimit.limit", 100)
	v.SetDefault("log.level", "info")
	v.SetDefault("log.encoding", "console")
	v.SetDefault("log.output_paths", []string{"stdout"})
	v.SetDefault("log.error_output_paths", []string{"stderr"})
	v.SetDefault("log.development", true)
	v.SetDefault("log.file.enabled", false)
	v.SetDefault("log.file.filename", "logs/user-center.log")
	v.SetDefault("log.file.max_size", 100)
	v.SetDefault("log.file.max_backups", 5)
	v.SetDefault("log.file.max_age", 30)
	v.SetDefault("log.file.local_time", true)
	v.SetDefault("log.file.compress", false)
	v.SetDefault("feature.enable_wechat_login", true)
	v.SetDefault("feature.enable_sms_login", true)
	v.SetDefault("feature.enable_debug_log", false)
	v.SetDefault("rag.enabled", false)
	v.SetDefault("rag.base_url", "http://127.0.0.1:18081")
	v.SetDefault("rag.timeout", "5s")
	v.SetDefault("rag.default_top_k", 3)
	v.SetDefault("rag.use_llm", true)
}

func bindEnvs(v *viper.Viper) {
	mustBindEnv(v, "db.dsn", "DB_DSN")
	mustBindEnv(v, "redis.addr", "REDIS_ADDR")
	mustBindEnv(v, "redis.password", "REDIS_PASSWORD")
	mustBindEnv(v, "redis.db", "REDIS_DB")
	mustBindEnv(v, "kafka.enabled", "KAFKA_ENABLED")
	mustBindEnv(v, "kafka.client_id", "KAFKA_CLIENT_ID")
	mustBindEnv(v, "kafka.consumer_group", "KAFKA_CLIENT_GROUP")
	mustBindEnv(v, "jwt.access_token_key", "JWT_ACCESS_TOKEN_KEY")
	mustBindEnv(v, "jwt.refresh_token_key", "JWT_REFRESH_TOKEN_KEY")
	mustBindEnv(v, "wechat.app_id", "WECHAT_APP_ID")
	mustBindEnv(v, "wechat.app_key", "WECHAT_APP_KEY")
	mustBindEnv(v, "wechat.state_token_key", "WECHAT_STATE_TOKEN_KEY")
	mustBindEnv(v, "rag.enabled", "RAG_ENABLED")
	mustBindEnv(v, "rag.base_url", "RAG_BASE_URL")
	mustBindEnv(v, "rag.timeout", "RAG_TIMEOUT")
	mustBindEnv(v, "rag.default_top_k", "RAG_DEFAULT_TOP_K")
	mustBindEnv(v, "rag.use_llm", "RAG_USE_LLM")
}

func mustBindEnv(v *viper.Viper, key string, envs ...string) {
	args := append([]string{key}, envs...)
	if err := v.BindEnv(args...); err != nil {
		panic(err)
	}
}

func validate(cfg AppConfig) error {
	if cfg.Server.Port <= 0 {
		return fmt.Errorf("server.port 必须大于 0")
	}
	if cfg.DB.DSN == "" {
		return fmt.Errorf("db.dsn 不能为空")
	}
	if cfg.Redis.Addr == "" {
		return fmt.Errorf("redis.addr 不能为空")
	}
	if cfg.Kafka.Enabled {
		if len(cfg.Kafka.Brokers) == 0 {
			return fmt.Errorf("kafka.brokers 不能为空")
		}
		if cfg.Kafka.ClientID == "" {
			return fmt.Errorf("kafka.client_id 不能为空")
		}
		if cfg.Kafka.ConsumerGroup == "" {
			return fmt.Errorf("kafka.consumer_group 不能为空")
		}
	}
	if cfg.JWT.AccessTokenKey == "" {
		return fmt.Errorf("jwt.access_token_key 不能为空")
	}
	if cfg.JWT.RefreshTokenKey == "" {
		return fmt.Errorf("jwt.refresh_token_key 不能为空")
	}
	if cfg.RAG.Enabled {
		if cfg.RAG.BaseURL == "" {
			return fmt.Errorf("rag.base_url 不能为空")
		}
		if cfg.RAG.Timeout <= 0 {
			return fmt.Errorf("rag.timeout 必须大于 0")
		}
		if cfg.RAG.DefaultTopK <= 0 {
			return fmt.Errorf("rag.default_top_k 必须大于 0")
		}
	}
	if cfg.Log.File.Enabled {
		if cfg.Log.File.Filename == "" {
			return fmt.Errorf("log.file.filename 不能为空")
		}
		if cfg.Log.File.MaxSize <= 0 {
			return fmt.Errorf("log.file.max_size 必须大于 0")
		}
		if cfg.Log.File.MaxBackups < 0 {
			return fmt.Errorf("log.file.max_backups 不能小于 0")
		}
		if cfg.Log.File.MaxAge < 0 {
			return fmt.Errorf("log.file.max_age 不能小于 0")
		}
	}
	if cfg.RateLimit.Enabled && cfg.RateLimit.Limit <= 0 {
		return fmt.Errorf("ratelimit.limit 必须大于 0")
	}
	if cfg.JWT.AccessTokenTTL <= 0 {
		return fmt.Errorf("jwt.access_token_ttl 必须大于 0")
	}
	if cfg.JWT.RefreshTokenTTL <= 0 {
		return fmt.Errorf("jwt.refresh_token_ttl 必须大于 0")
	}
	if cfg.JWT.IdleTimeout <= 0 {
		return fmt.Errorf("jwt.idle_timeout 必须大于 0")
	}
	if cfg.JWT.AbsoluteTimeout <= 0 {
		return fmt.Errorf("jwt.absolute_timeout 必须大于 0")
	}
	if cfg.Feature.EnableWechatLogin {
		if cfg.Wechat.AppID == "" {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.app_id 不能为空")
		}
		if cfg.Wechat.AppKey == "" {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.app_key 不能为空")
		}
		if cfg.Wechat.RedirectURL == "" {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.redirect_url 不能为空")
		}
		if cfg.Wechat.StateCookieName == "" {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.state_cookie_name 不能为空")
		}
		if cfg.Wechat.StateTokenKey == "" {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.state_token_key 不能为空")
		}
		if cfg.Wechat.StateTokenTTL <= 0 {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.state_token_ttl 必须大于 0")
		}
		if cfg.Wechat.StateCookiePath == "" {
			return fmt.Errorf("feature.enable_wechat_login=true 时，wechat.state_cookie_path 不能为空")
		}
	}
	return nil
}
