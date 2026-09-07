package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
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
		panic("kafka.enabled=false，worker 无法启动")
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
	db := ioc.InitDB(&cfg)
	group := ioc.InitKafkaConsumerGroup(&cfg)
	defer func() {
		_ = group.Close()
	}()

	pointRepo := repository.NewPointRepositoryImpl(dao.NewGORMPointDAO(db))
	registeredDeduper := worker.NewRedisDeduplicator(rdb, "worker:user_registered")
	notePublishedDeduper := worker.NewRedisDeduplicator(rdb, worker.NotePublishedDeduperNamespace)
	activityProcessor := worker.NewRedisUserActivityProcessor(rdb)
	followRepo := repository.NewFollowRepositoryImpl(dao.NewGORMFollowDAO(db))
	noteRepo := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	feedInbox := cache.NewRedisFeedInbox(rdb, cfg.Feed.InboxLimit())
	feedSvc := service.NewFeedServiceImpl(feedInbox, noteRepo, followRepo, cfg.Feed.FanoutBatch(), cfg.Feed.CelebrityThreshold(), appLogger)

	registeredHandler := worker.NewUserRegisteredHandler(pointRepo, registeredDeduper, appLogger)
	activityHandler := worker.NewUserActivityHandler(activityProcessor, appLogger)
	notePublishedHandler := worker.NewNotePublishedHandler(feedSvc, notePublishedDeduper, appLogger)
	consumerHandler := worker.NewConsumerGroupHandler(appLogger, map[string]worker.MessageHandler{
		events.TopicUserRegistered: registeredHandler.Handle,
		events.TopicUserActivity:   activityHandler.Handle,
		events.TopicNotePublished:  notePublishedHandler.Handle,
	})

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	zapLogger.Info("Kafka worker 启动成功",
		zap.Strings("brokers", cfg.Kafka.Brokers),
		zap.String("group", cfg.Kafka.ConsumerGroup),
	)

	for {
		if err = group.Consume(ctx, []string{
			events.TopicUserRegistered,
			events.TopicUserActivity,
			events.TopicNotePublished,
		}, consumerHandler); err != nil {
			if ctx.Err() != nil {
				return
			}
			appLogger.Error("Kafka 消费循环异常",
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
