package ioc

import (
	"user-center/internal/config"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
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

	zapCfg := zap.Config{
		Level:             atomicLevel,
		Development:       cfg.Development,
		Encoding:          cfg.Encoding,
		EncoderConfig:     encoderCfg,
		OutputPaths:       cfg.OutputPaths,
		ErrorOutputPaths:  cfg.ErrorOutputPaths,
		DisableCaller:     cfg.DisableCaller,
		DisableStacktrace: cfg.DisableStacktrace,
	}
	logger, err := zapCfg.Build()
	if err != nil {
		return nil, nil, err
	}
	zap.ReplaceGlobals(logger)
	return logger, &atomicLevel, nil
}
