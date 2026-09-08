package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"
	"user-center/internal/events"
	"user-center/internal/repository"
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
	if !cfg.Kafka.Enabled || !cfg.Search.Enabled {
		panic("kafka.enabled 和 search.enabled 必须为 true")
	}
	zapLogger, _, err := ioc.InitLogger(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = zapLogger.Sync() }()
	appLogger := ioc.NewLogger(zapLogger)
	if err = ioc.EnsureKafkaTopics(&cfg, appLogger); err != nil {
		panic(err)
	}

	db := ioc.InitDB(&cfg)
	notes := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	index := ioc.InitNoteSearchIndex(&cfg)
	if err = index.EnsureIndex(context.Background()); err != nil {
		panic(err)
	}
	indexer := service.NewSearchIndexService(notes, notes, index, cfg.Search.ReindexBatchSize)
	handler := worker.NewSearchIndexHandler(indexer, appLogger)
	consumerHandler := worker.NewConsumerGroupHandler(appLogger, map[string]worker.MessageHandler{
		events.TopicNotePublished: handler.HandlePublished,
		events.TopicNoteDeleted:   handler.HandleDeleted,
	})
	group := ioc.InitSearchKafkaConsumerGroup(&cfg)
	defer func() { _ = group.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	zapLogger.Info("搜索索引 worker 启动成功",
		zap.Strings("brokers", cfg.Kafka.Brokers),
		zap.String("group", cfg.Search.ConsumerGroup),
	)
	for {
		if err = group.Consume(ctx, []string{events.TopicNotePublished, events.TopicNoteDeleted}, consumerHandler); err != nil {
			if ctx.Err() != nil {
				return
			}
			appLogger.Error("搜索索引消费循环异常", logger.Error(err))
			time.Sleep(time.Second)
			continue
		}
		if ctx.Err() != nil {
			return
		}
	}
}
