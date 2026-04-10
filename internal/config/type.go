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
	JWT       JWTConfig       `mapstructure:"jwt"`
	Wechat    WechatConfig    `mapstructure:"wechat"`
	CORS      CORSConfig      `mapstructure:"cors"`
	RateLimit RateLimitConfig `mapstructure:"ratelimit"`
	Log       LogConfig       `mapstructure:"log"`
	Feature   FeatureConfig   `mapstructure:"feature"`
}

func (conf AppConfig) Addr() string {
	return fmt.Sprintf(":%d", conf.Server.Port)
}

type ServerConfig struct {
	Name string `mapstructure:"name"`
	Port int    `mapstructure:"port"`
	Mode string `mapstructure:"mode"`
}

type DBConfig struct {
	DSN string `mapstructure:"dsn"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
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

type DynamicConfig struct {
	LogLevel string
	Feature  FeatureConfig
}
