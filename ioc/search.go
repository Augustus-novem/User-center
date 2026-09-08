package ioc

import (
	"fmt"
	"user-center/internal/config"
	searchintegration "user-center/internal/integration/search"
	"user-center/internal/repository"
	"user-center/internal/service"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

func InitNoteSearchIndex(cfg *config.AppConfig) repository.NoteSearchIndex {
	client, err := searchintegration.NewElasticsearch(cfg.Search.Address, cfg.Search.Index, cfg.Search.RequestTimeout)
	if err != nil {
		panic(fmt.Sprintf("Elasticsearch client 初始化失败: %v", err))
	}
	return client
}

func InitSearchService(cfg *config.AppConfig, index repository.NoteSearchIndex, notes *repository.CachedNoteRepository, l logger.Logger) service.SearchService {
	return service.NewSearchServiceImpl(
		cfg.Search.Enabled,
		index,
		notes,
		cfg.Search.RequestTimeout,
		cfg.Search.DBFallbackTimeout,
		cfg.Search.FallbackWindow,
		cfg.Search.MaxLimit,
		l,
	)
}

func InitSearchKafkaConsumerGroup(cfg *config.AppConfig) sarama.ConsumerGroup {
	if !cfg.Kafka.Enabled {
		panic("kafka is disabled")
	}
	consumer, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Search.ConsumerGroup, newSaramaConfig(cfg.Kafka))
	if err != nil {
		panic(err)
	}
	return consumer
}
