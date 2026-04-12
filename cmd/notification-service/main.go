package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user-center/internal/events"
	"user-center/internal/notification"
	"user-center/internal/repository"
	"user-center/internal/worker"
	"user-center/ioc"
	"user-center/pkg/logger"

	"go.uber.org/zap"
)

func main() {
	cfgManager, err := ioc.InitConfig()
	if err != nil {
		panic(err)
	}
	cfg := cfgManager.App()
	if !cfg.Kafka.Enabled {
		panic("kafka.enabled=false，notification-service 无法启动")
	}
	zapLogger, _, err := ioc.InitLogger(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() {
		_ = zapLogger.Sync()
	}()
	appLogger := ioc.NewLogger(zapLogger)
	if err = ioc.EnsureKafkaTopics(&cfg, appLogger); err != nil {
		panic(err)
	}

	rdb := ioc.InitRedis(&cfg)
	group := ioc.InitKafkaConsumerGroup(&cfg)
	defer func() {
		_ = group.Close()
	}()

	welcomeMessageRepo := repository.NewRedisWelcomeMessageRepository(rdb)
	deduper := worker.NewRedisDeduplicator(rdb, "notification:user_registered")
	registeredHandler := notification.NewUserRegisteredHandler(welcomeMessageRepo, deduper, appLogger)
	consumerHandler := worker.NewConsumerGroupHandler(appLogger, map[string]worker.MessageHandler{
		events.TopicUserRegistered: registeredHandler.Handle,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	zapLogger.Info("Kafka notification-service 启动成功",
		zap.Strings("brokers", cfg.Kafka.Brokers),
		zap.String("group", cfg.Kafka.ConsumerGroup),
	)

	for {
		if err = group.Consume(ctx, []string{events.TopicUserRegistered}, consumerHandler); err != nil {
			if ctx.Err() != nil {
				return
			}
			appLogger.Error("notification-service 消费循环异常",
				logger.Field{Key: "error", Value: err},
			)
			time.Sleep(time.Second)
			continue
		}
		if ctx.Err() != nil {
			return
		}
	}
}
