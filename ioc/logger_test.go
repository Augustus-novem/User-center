package ioc

import (
	"testing"
	appconfig "user-center/internal/config"
)

func TestInitLogger(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		logger, level, err := InitLogger(appconfig.LogConfig{
			Level:            "debug",
			Encoding:         "console",
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			Development:      true,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logger == nil || level == nil {
			t.Fatal("logger and atomic level should not be nil")
		}
	})

	t.Run("lumberjack file enabled", func(t *testing.T) {
		logger, level, err := InitLogger(appconfig.LogConfig{
			Level:            "info",
			Encoding:         "json",
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
			File: appconfig.LogFileConfig{
				Enabled:    true,
				Filename:   t.TempDir() + "/app.log",
				MaxSize:    1,
				MaxBackups: 1,
				MaxAge:     1,
				LocalTime:  true,
				Compress:   false,
			},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if logger == nil || level == nil {
			t.Fatal("logger and atomic level should not be nil")
		}
	})

	t.Run("invalid level", func(t *testing.T) {
		logger, level, err := InitLogger(appconfig.LogConfig{
			Level:            "not-a-level",
			Encoding:         "console",
			OutputPaths:      []string{"stdout"},
			ErrorOutputPaths: []string{"stderr"},
		})
		if err == nil {
			t.Fatal("expected invalid level error, got nil")
		}
		if logger != nil || level != nil {
			t.Fatal("logger and level should be nil on error")
		}
	})
}
