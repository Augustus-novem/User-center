package main

import (
	"os"
	"user-center/ioc"

	"go.uber.org/zap"
)

func main() {
	cfgManager, err := ioc.InitConfig()
	if err != nil {
		panic(err)
	}
	cfg := cfgManager.App()
	logger, atomicLevel, err := ioc.InitLogger(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	cfgManager.StartWatch(logger, atomicLevel)
	logger.Info("配置初始化完成", zap.String("config", cfgManager.Path()))
	server := InitWebServer(&cfg, cfgManager)
	logger.Info("HTTP 服务启动", zap.String("addr", cfg.Addr()))
	if err = server.Run(cfg.Addr()); err != nil {
		logger.Error("HTTP 服务启动失败", zap.Error(err))
		os.Exit(1)
	}
}
