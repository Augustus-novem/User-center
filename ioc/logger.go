package ioc

import (
	"fmt"
	"os"
	"path/filepath"
	"user-center/internal/config"
	"user-center/pkg/logger"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

func InitLogger(cfg config.LogConfig) (*zap.Logger, *zap.AtomicLevel, error) {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		return nil, nil, err
	}
	atomicLevel := zap.NewAtomicLevelAt(level)

	encoderCfg := zap.NewProductionEncoderConfig()
	if cfg.Development {
		encoderCfg = zap.NewDevelopmentEncoderConfig()
	}
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	var encoder zapcore.Encoder
	switch cfg.Encoding {
	case "console":
		encoder = zapcore.NewConsoleEncoder(encoderCfg)
	default:
		encoder = zapcore.NewJSONEncoder(encoderCfg)
	}

	output, err := buildOutputSyncer(cfg)
	if err != nil {
		return nil, nil, err
	}
	errorOutput, err := buildErrorOutputSyncer(cfg)
	if err != nil {
		return nil, nil, err
	}

	core := zapcore.NewCore(encoder, output, atomicLevel)

	opts := make([]zap.Option, 0, 4)
	if !cfg.DisableCaller {
		opts = append(opts, zap.AddCaller())
	}
	if !cfg.DisableStacktrace {
		opts = append(opts, zap.AddStacktrace(zap.ErrorLevel))
	}
	if cfg.Development {
		opts = append(opts, zap.Development())
	}
	opts = append(opts, zap.ErrorOutput(errorOutput))

	log := zap.New(core, opts...)
	zap.ReplaceGlobals(log)
	return log, &atomicLevel, nil
}

func NewLogger(log *zap.Logger) logger.Logger {
	if log == nil {
		return logger.NoOpLogger{}
	}
	return logger.NewZapLogger(log)
}

func buildOutputSyncer(cfg config.LogConfig) (zapcore.WriteSyncer, error) {
	syncers := make([]zapcore.WriteSyncer, 0, len(cfg.OutputPaths)+1)
	for _, path := range cfg.OutputPaths {
		sink, err := openSink(path)
		if err != nil {
			return nil, err
		}
		syncers = append(syncers, sink)
	}

	if cfg.File.Enabled {
		if err := ensureLogDir(cfg.File.Filename); err != nil {
			return nil, err
		}
		syncers = append(syncers, zapcore.AddSync(&lumberjack.Logger{
			Filename:   cfg.File.Filename,
			MaxSize:    cfg.File.MaxSize,
			MaxBackups: cfg.File.MaxBackups,
			MaxAge:     cfg.File.MaxAge,
			LocalTime:  cfg.File.LocalTime,
			Compress:   cfg.File.Compress,
		}))
	}

	if len(syncers) == 0 {
		syncers = append(syncers, zapcore.AddSync(os.Stdout))
	}
	return zapcore.NewMultiWriteSyncer(syncers...), nil
}

func buildErrorOutputSyncer(cfg config.LogConfig) (zapcore.WriteSyncer, error) {
	syncers := make([]zapcore.WriteSyncer, 0, len(cfg.ErrorOutputPaths))
	for _, path := range cfg.ErrorOutputPaths {
		sink, err := openSink(path)
		if err != nil {
			return nil, err
		}
		syncers = append(syncers, sink)
	}
	if len(syncers) == 0 {
		syncers = append(syncers, zapcore.AddSync(os.Stderr))
	}
	return zapcore.NewMultiWriteSyncer(syncers...), nil
}

func openSink(path string) (zapcore.WriteSyncer, error) {
	switch path {
	case "stdout":
		return zapcore.AddSync(os.Stdout), nil
	case "stderr":
		return zapcore.AddSync(os.Stderr), nil
	default:
		if err := ensureLogDir(path); err != nil {
			return nil, err
		}
		file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			return nil, fmt.Errorf("open log sink %s: %w", path, err)
		}
		return zapcore.AddSync(file), nil
	}
}

func ensureLogDir(filename string) error {
	dir := filepath.Dir(filename)
	if dir == "." || dir == "" {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create log dir %s: %w", dir, err)
	}
	return nil
}
