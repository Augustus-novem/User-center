package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
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
	defer func() { _ = zapLogger.Sync() }()
	cfgManager.StartWatch(zapLogger, atomicLevel)
	appLogger := ioc.NewLogger(zapLogger)
	zapLogger.Info("配置初始化完成", zap.String("config", cfgManager.Path()))

	appCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var relayCancel context.CancelFunc
	var relayClose func()
	if cfg.Kafka.Enabled {
		db := ioc.InitDB(&cfg)
		relay := ioc.InitEventRelay(&cfg, db, appLogger)
		if relay != nil {
			var relayCtx context.Context
			relayCtx, relayCancel = context.WithCancel(appCtx)
			relayClose = func() { _ = relay.Close() }
			go relay.Run(relayCtx, time.Second)
			zapLogger.Info("Outbox relay 启动成功", zap.Strings("brokers", cfg.Kafka.Brokers))
		}
	}
	defer func() {
		if relayCancel != nil {
			relayCancel()
		}
		if relayClose != nil {
			relayClose()
		}
	}()

	engine := InitWebServer(&cfg, cfgManager, appLogger)
	server := &http.Server{
		Addr:              cfg.Addr(),
		Handler:           engine,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}
	serverErr := make(chan error, 1)
	go func() {
		zapLogger.Info("HTTP 服务启动", zap.String("addr", cfg.Addr()))
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err = <-serverErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			panic(err)
		}
		return
	case <-appCtx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	if err = server.Shutdown(shutdownCtx); err != nil {
		zapLogger.Error("HTTP 服务优雅关闭失败", zap.Error(err))
		_ = server.Close()
	}
}
