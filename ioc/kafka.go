package ioc

import (
	"fmt"
	"sort"
	"user-center/internal/config"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/internal/repository/dao"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	"gorm.io/gorm"
)

func InitEventPublisher(cfg *config.AppConfig, outboxRepo repository.EventOutboxRepository, l logger.Logger) events.Publisher {
	if !cfg.Kafka.Enabled {
		l.Warn("Kafka 未启用，事件发布改为非 Kafka 模式")
		return events.NopPublisher{}
	}
	return events.NewOutboxPublisher(outboxRepo, l)
}

func InitEventRelay(cfg *config.AppConfig, db *gorm.DB, l logger.Logger) *events.OutboxRelay {
	if !cfg.Kafka.Enabled {
		return nil
	}
	if err := EnsureKafkaTopics(cfg, l); err != nil {
		panic(fmt.Sprintf("Kafka topic 初始化失败: %v", err))
	}
	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, newSaramaConfig(cfg.Kafka))
	if err != nil {
		panic(fmt.Sprintf("Kafka relay producer 初始化失败: %v", err))
	}
	outboxRepo := repository.NewEventOutboxRepositoryImpl(dao.NewGORMEventOutboxDAO(db))
	return events.NewOutboxRelay(outboxRepo, producer, l)
}

func InitKafkaConsumerGroup(cfg *config.AppConfig) sarama.ConsumerGroup {
	if !cfg.Kafka.Enabled {
		panic("kafka is disabled")
	}
	consumer, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, cfg.Kafka.ConsumerGroup, newSaramaConfig(cfg.Kafka))
	if err != nil {
		panic(err)
	}
	return consumer
}

func EnsureKafkaTopics(cfg *config.AppConfig, l logger.Logger) error {
	if !cfg.Kafka.Enabled {
		return nil
	}
	admin, err := sarama.NewClusterAdmin(cfg.Kafka.Brokers, newSaramaConfig(cfg.Kafka))
	if err != nil {
		return err
	}
	defer func() { _ = admin.Close() }()

	topics, err := admin.ListTopics()
	if err != nil {
		return err
	}
	need := []string{events.TopicUserRegistered, events.TopicUserActivity}
	sort.Strings(need)
	for _, topic := range need {
		if _, ok := topics[topic]; ok {
			continue
		}
		err = admin.CreateTopic(topic, &sarama.TopicDetail{
			NumPartitions:     1,
			ReplicationFactor: 1,
		}, false)
		if err != nil && err != sarama.ErrTopicAlreadyExists {
			return fmt.Errorf("create topic %s: %w", topic, err)
		}
		l.Info("Kafka topic 已确保存在", logger.Field{Key: "topic", Value: topic})
	}
	return nil
}

func newSaramaConfig(cfg config.KafkaConfig) *sarama.Config {
	scfg := sarama.NewConfig()
	scfg.ClientID = cfg.ClientID
	scfg.Producer.Return.Successes = true
	scfg.Producer.RequiredAcks = sarama.WaitForAll
	scfg.Producer.Retry.Max = 3
	scfg.Consumer.Return.Errors = true
	scfg.Consumer.Offsets.Initial = sarama.OffsetOldest
	return scfg
}
