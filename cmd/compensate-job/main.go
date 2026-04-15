package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user-center/internal/config"
	"user-center/internal/repository/cache"
	"user-center/internal/service"
	"user-center/ioc"
	"user-center/pkg/logger"
)

// 幂等键补偿定时任务
func main() {
	cfg := config.MustLoad()
	l := ioc.InitLogger(cfg)
	
	l.Info("幂等键补偿任务启动")
	
	// 初始化 Redis
	redisClient := ioc.InitRedis(cfg)
	
	// 初始化幂等缓存
	idempotentCache := cache.NewRedisIdempotentCache(redisClient, "idempotent")
	
	// TODO: 初始化处理记录仓储（需要实现）
	// processRecordRepo := repository.NewProcessRecordRepository(db)
	
	// 初始化补偿服务
	// compensateService := service.NewIdempotentCompensateService(
	// 	idempotentCache,
	// 	processRecordRepo,
	// 	l,
	// )
	
	// 创建定时器，每小时执行一次
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	
	// 监听退出信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	
	// 立即执行一次
	l.Info("开始首次补偿")
	if err := runCompensate(context.Background(), idempotentCache, l); err != nil {
		l.Error("补偿失败", logger.Field{Key: "error", Value: err})
	}
	
	for {
		select {
		case <-ticker.C:
			l.Info("开始定时补偿")
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
			if err := runCompensate(ctx, idempotentCache, l); err != nil {
				l.Error("补偿失败", logger.Field{Key: "error", Value: err})
			}
			cancel()
		case <-quit:
			l.Info("收到退出信号，停止补偿任务")
			return
		}
	}
}

func runCompensate(ctx context.Context, idempotentCache cache.IdempotentCache, l logger.Logger) error {
	// TODO: 实现具体的补偿逻辑
	// compensateService.CompensateExpiredKeys(ctx)
	
	l.Info("补偿任务执行完成")
	return nil
}

func init() {
	// 设置时区
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		fmt.Printf("加载时区失败: %v\n", err)
		os.Exit(1)
	}
	time.Local = loc
}
