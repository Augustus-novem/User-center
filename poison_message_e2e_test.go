//go:build e2e

package main

import (
	"bytes"
	"context"
	"testing"
	"time"

	"user-center/internal/events"
	"user-center/internal/worker"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
)

type poisonActivityProcessor struct{}

func (poisonActivityProcessor) ProcessOnce(context.Context, events.UserActivityEvent) (bool, error) {
	return true, nil
}

func TestKafkaPoisonMessageRoutesToDLQE2E(t *testing.T) {
	cfg := loadE2EConfig(t)
	pingE2EDeps(t, cfg)
	topic := "community-poison-e2e-" + uuid.NewString()
	dlqTopic := topic + worker.DeadLetterSuffix

	admin, err := sarama.NewClusterAdmin(cfg.Kafka.Brokers, sarama.NewConfig())
	if err != nil {
		t.Skipf("kafka admin unavailable: %v", err)
	}
	for _, name := range []string{topic, dlqTopic} {
		if err = admin.CreateTopic(name, &sarama.TopicDetail{NumPartitions: 1, ReplicationFactor: 1}, false); err != nil {
			_ = admin.Close()
			t.Fatalf("create topic %s: %v", name, err)
		}
	}
	t.Cleanup(func() {
		_ = admin.DeleteTopic(topic)
		_ = admin.DeleteTopic(dlqTopic)
		_ = admin.Close()
	})

	producerConfig := sarama.NewConfig()
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, producerConfig)
	if err != nil {
		t.Fatal(err)
	}

	consumer, err := sarama.NewConsumer(cfg.Kafka.Brokers, sarama.NewConfig())
	if err != nil {
		_ = producer.Close()
		t.Fatal(err)
	}
	dlq, err := consumer.ConsumePartition(dlqTopic, 0, sarama.OffsetNewest)
	if err != nil {
		_ = consumer.Close()
		_ = producer.Close()
		t.Fatal(err)
	}

	groupConfig := sarama.NewConfig()
	groupConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	group, err := sarama.NewConsumerGroup(cfg.Kafka.Brokers, "community-poison-e2e-"+uuid.NewString(), groupConfig)
	if err != nil {
		_ = dlq.Close()
		_ = consumer.Close()
		_ = producer.Close()
		t.Fatal(err)
	}
	consumeCtx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() {
		cancel()
		_ = group.Close()
		_ = dlq.Close()
		_ = consumer.Close()
		_ = producer.Close()
	})

	activity := worker.NewUserActivityHandler(poisonActivityProcessor{}, logger.NewNoOpLogger())
	handler := worker.NewConsumerGroupHandlerWithDLQ(logger.NewNoOpLogger(), producer, map[string]worker.MessageHandler{
		topic: activity.Handle,
	})
	consumeErr := make(chan error, 1)
	go func() {
		consumeErr <- group.Consume(consumeCtx, []string{topic}, handler)
	}()

	payload := []byte(`{"event_id":`)
	if _, _, err = producer.SendMessage(&sarama.ProducerMessage{Topic: topic, Value: sarama.ByteEncoder(payload)}); err != nil {
		t.Fatal(err)
	}
	select {
	case msg := <-dlq.Messages():
		if msg == nil || msg.Topic != dlqTopic || !bytes.Equal(msg.Value, payload) {
			t.Fatalf("unexpected DLQ message: %+v", msg)
		}
	case err = <-dlq.Errors():
		t.Fatal(err)
	case err = <-consumeErr:
		if err != nil {
			t.Fatalf("consumer exited before DLQ delivery: %v", err)
		}
		t.Fatal("consumer exited before DLQ delivery")
	case <-time.After(20 * time.Second):
		t.Fatal("timed out waiting for poison message in DLQ")
	}
}
