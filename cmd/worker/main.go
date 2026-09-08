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
	dlqProducer := ioc.InitKafkaSyncProducer(&cfg)
	defer func() { _ = dlqProducer.Close() }()

	pointRepo := repository.NewPointRepositoryImpl(dao.NewGORMPointDAO(db))
	registeredDeduper := worker.NewRedisDeduplicator(rdb, "worker:user_registered")
	notePublishedDeduper := worker.NewRedisDeduplicator(rdb, worker.NotePublishedDeduperNamespace)
	activityProcessor := worker.NewRedisUserActivityProcessor(rdb)
	followRepo := repository.NewFollowRepositoryImpl(dao.NewGORMFollowDAO(db))
	noteRepo := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	feedInbox := cache.NewRedisFeedInbox(rdb, cfg.Feed.InboxLimit())
	feedSvc := service.NewFeedServiceImpl(feedInbox, noteRepo, followRepo, cfg.Feed.FanoutBatch(), cfg.Feed.CelebrityThreshold(), appLogger)
	hotCache := cache.NewRedisHotRankCache(rdb,
		time.Duration(cfg.HotRank.WindowMinutes)*time.Minute,
		cfg.HotRank.SnapshotTTL,
		cfg.HotRank.EventDedupTTL,
	)
	hotRepo := repository.NewHotRankRepositoryImpl(hotCache)
	hotSvc := service.NewHotRankServiceImpl(hotRepo, service.HotRankWeights{
		Publish: cfg.HotRank.PublishWeight,
		Like:    cfg.HotRank.LikeWeight,
		Comment: cfg.HotRank.CommentWeight,
	})

	registeredHandler := worker.NewUserRegisteredHandler(pointRepo, registeredDeduper, appLogger)
	activityHandler := worker.NewUserActivityHandler(activityProcessor, appLogger)
	notePublishedHandler := worker.NewNotePublishedHandler(feedSvc, notePublishedDeduper, appLogger)
	hotRankHandler := worker.NewHotRankHandler(hotSvc, appLogger)
	consumerHandler := worker.NewConsumerGroupHandlerWithDLQ(appLogger, dlqProducer, map[string]worker.MessageHandler{
		events.TopicUserRegistered: registeredHandler.Handle,
		events.TopicUserActivity:   activityHandler.Handle,
		events.TopicNotePublished:  worker.ChainHandlers(notePublishedHandler.Handle, hotRankHandler.Handle),
		events.TopicNoteLiked:      hotRankHandler.Handle,
		events.TopicCommentCreated: hotRankHandler.Handle,
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
			events.TopicNoteLiked,
			events.TopicCommentCreated,
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
