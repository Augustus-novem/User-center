package config

import (
	"fmt"
	"time"
)

type DynamicProvider interface {
	Dynamic() DynamicConfig
}

type AppConfig struct {
	Server    ServerConfig    `mapstructure:"server"`
	DB        DBConfig        `mapstructure:"db"`
	Redis     RedisConfig     `mapstructure:"redis"`
	Kafka     KafkaConfig     `mapstructure:"kafka"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Wechat    WechatConfig    `mapstructure:"wechat"`
	CORS      CORSConfig      `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
	Log       LogConfig       `mapstructure:"log"`
	Feature   FeatureConfig   `mapstructure:"feature"`
	Feed      FeedConfig      `mapstructure:"feed"`
	HotRank   HotRankConfig   `mapstructure:"hot_rank"`
	Search    SearchConfig    `mapstructure:"search"`
}

func (conf AppConfig) Addr() string {
	return fmt.Sprintf(":%d", conf.Server.Port)
}

type ServerConfig struct {
	Name                string        `mapstructure:"name"`
	Port                int           `mapstructure:"port"`
	Mode                string        `mapstructure:"mode"`
	ReadHeaderTimeout   time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout         time.Duration `mapstructure:"read_timeout"`
	WriteTimeout        time.Duration `mapstructure:"write_timeout"`
	IdleTimeout         time.Duration `mapstructure:"idle_timeout"`
	ShutdownTimeout     time.Duration `mapstructure:"shutdown_timeout"`
	MaxRequestBodyBytes int64         `mapstructure:"max_request_body_bytes"`
}

type DBConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Enabled       bool     `mapstructure:"enabled"`
	Brokers       []string `mapstructure:"brokers"`
	ClientID      string   `mapstructure:"client_id"`
	ConsumerGroup string   `mapstructure:"consumer_group"`
}

type JWTConfig struct {
	AccessTokenKey  string        `mapstructure:"access_token_key"`
	RefreshTokenKey string        `mapstructure:"refresh_token_key"`
	AccessTokenTTL  time.Duration `mapstructure:"access_token_ttl"`
	RefreshTokenTTL time.Duration `mapstructure:"refresh_token_ttl"`
	IdleTimeout     time.Duration `mapstructure:"idle_timeout"`
	AbsoluteTimeout time.Duration `mapstructure:"absolute_timeout"`
}

type WechatConfig struct {
	AppID           string        `mapstructure:"app_id"`
	AppKey          string        `mapstructure:"app_key"`
	RedirectURL     string        `mapstructure:"redirect_url"`
	StateCookieName string        `mapstructure:"state_cookie_name"`
	StateTokenKey   string        `mapstructure:"state_token_key"`
	StateTokenTTL   time.Duration `mapstructure:"state_token_ttl"`
	StateCookiePath string        `mapstructure:"state_cookie_path"`
	HTTPTimeout     time.Duration `mapstructure:"http_timeout"`
}

type CORSConfig struct {
	AllowCredentials bool          `mapstructure:"allow_credentials"`
	AllowOrigins     []string      `mapstructure:"allow_origins"`
	AllowMethods     []string      `mapstructure:"allow_methods"`
	AllowHeaders     []string      `mapstructure:"allow_headers"`
	ExposeHeaders    []string      `mapstructure:"expose_headers"`
	MaxAge           time.Duration `mapstructure:"max_age"`
}

type RateLimitConfig struct {
	Enabled  bool          `mapstructure:"enabled"`
	Prefix   string        `mapstructure:"prefix"`
	Interval time.Duration `mapstructure:"interval"`
	Limit    int           `mapstructure:"limit"`
}

type LogFileConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	LocalTime  bool   `mapstructure:"local_time"`
	Compress   bool   `mapstructure:"compress"`
}

type LogConfig struct {
	Level             string        `mapstructure:"level"`
	Encoding          string        `mapstructure:"encoding"`
	OutputPaths       []string      `mapstructure:"output_paths"`
	ErrorOutputPaths  []string      `mapstructure:"error_output_paths"`
	DisableCaller     bool          `mapstructure:"disable_caller"`
	DisableStacktrace bool          `mapstructure:"disable_stacktrace"`
	Development       bool          `mapstructure:"development"`
	File              LogFileConfig `mapstructure:"file"`
}

type FeatureConfig struct {
	EnableWechatLogin bool `mapstructure:"enable_wechat_login"`
	EnableSMSLogin    bool `mapstructure:"enable_sms_login"`
	EnableDebugLog    bool `mapstructure:"enable_debug_log"`
}

type FeedConfig struct {
	FanoutBatchSize int `mapstructure:"fanout_batch_size"`
	InboxMaxItems   int `mapstructure:"inbox_max_items"`
	FanoutThreshold int `mapstructure:"fanout_threshold"`
}

type HotRankConfig struct {
	WindowMinutes int           `mapstructure:"window_minutes"`
	SnapshotTTL   time.Duration `mapstructure:"snapshot_ttl"`
	EventDedupTTL time.Duration `mapstructure:"event_dedup_ttl"`
	PublishWeight int64         `mapstructure:"publish_weight"`
	LikeWeight    int64         `mapstructure:"like_weight"`
	CommentWeight int64         `mapstructure:"comment_weight"`
}

type SearchConfig struct {
	Enabled           bool          `mapstructure:"enabled"`
	Address           string        `mapstructure:"address"`
	Index             string        `mapstructure:"index"`
	ConsumerGroup     string        `mapstructure:"consumer_group"`
	RequestTimeout    time.Duration `mapstructure:"request_timeout"`
	DBFallbackTimeout time.Duration `mapstructure:"db_fallback_timeout"`
	FallbackWindow    time.Duration `mapstructure:"fallback_window"`
	MaxLimit          int           `mapstructure:"max_limit"`
	ReindexBatchSize  int           `mapstructure:"reindex_batch_size"`
}

func (c FeedConfig) FanoutBatch() int {
	if c.FanoutBatchSize <= 0 {
		return 200
	}
	return c.FanoutBatchSize
}

func (c FeedConfig) InboxLimit() int {
	if c.InboxMaxItems <= 0 {
		return 500
	}
	return c.InboxMaxItems
}

func (c FeedConfig) CelebrityThreshold() int {
	if c.FanoutThreshold <= 0 {
		return 0
	}
	return c.FanoutThreshold
}

type DynamicConfig struct {
	LogLevel string
	Feature  FeatureConfig
}
