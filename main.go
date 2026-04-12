package main

import (
	"context"
	"os"
	"time"
	"user-center/ioc"

	"go.uber.org/zap"
)

func main() {
	cfgManager, err := ioc.InitConfig()
	if err != nil {
		panic(err)
	}
	cfg := cfgManager.App()

	zapLogger, atomicLevel, err := ioc.InitLogger(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = zapLogger.Sync()
	}()

	cfgManager.StartWatch(zapLogger, atomicLevel)

	appLogger := ioc.NewLogger(zapLogger)
	zapLogger.Info("配置初始化完成", zap.String("config", cfgManager.Path()))

	var relayCancel context.CancelFunc
	var relayClose func()

	if cfg.Kafka.Enabled {
		db := ioc.InitDB(&cfg)
		relay := ioc.InitEventRelay(&cfg, db, appLogger)
		if relay != nil {
			ctx, cancel := context.WithCancel(context.Background())
			relayCancel = cancel
			relayClose = func() {
				_ = relay.Close()
			}
			go relay.Run(ctx, time.Second)
			zapLogger.Info("Outbox relay 启动成功",
				zap.Strings("brokers", cfg.Kafka.Brokers),
			)
		}
	}

	server := InitWebServer(&cfg, cfgManager, appLogger)

	zapLogger.Info("HTTP 服务启动", zap.String("addr", cfg.Addr()))
	if err = server.Run(cfg.Addr()); err != nil {
		zapLogger.Error("HTTP 服务启动失败", zap.Error(err))
		if relayCancel != nil {
			relayCancel()
		}
		if relayClose != nil {
			relayClose()
		}
		os.Exit(1)
	}
}
