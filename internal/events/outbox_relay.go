package events

import (
	"context"
	"fmt"
	"time"
	"user-center/internal/repository"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type OutboxRelay struct {
	repo      repository.EventOutboxRepository
	producer  sarama.SyncProducer
	logger    logger.Logger
	batchSize int
}

func NewOutboxRelay(repo repository.EventOutboxRepository, producer sarama.SyncProducer, l logger.Logger) *OutboxRelay {
	return &OutboxRelay{
		repo:      repo,
		producer:  producer,
		logger:    l,
		batchSize: 100,
	}
}

func (r *OutboxRelay) DispatchBatch(ctx context.Context) error {
	msgs, err := r.repo.ListPending(ctx, r.batchSize)
	if err != nil {
		return err
	}
	for _, msg := range msgs {
		_, _, sendErr := r.producer.SendMessage(&sarama.ProducerMessage{
			Topic: msg.Topic,
			Key:   sarama.StringEncoder(msg.MessageKey),
			Value: sarama.ByteEncoder(msg.Payload),
			Headers: []sarama.RecordHeader{
				{Key: []byte("content-type"), Value: []byte("application/json")},
			},
		})
		if sendErr != nil {
			markErr := r.repo.MarkFailed(ctx, msg.ID, sendErr.Error())
			if markErr != nil {
				return fmt.Errorf("send outbox message failed: %w; mark failed failed: %v", sendErr, markErr)
			}
			r.logger.Warn("outbox 事件投递失败，等待下轮重试",
				logger.Field{Key: "outbox_id", Value: msg.ID},
				logger.Field{Key: "topic", Value: msg.Topic},
				logger.Field{Key: "attempts", Value: msg.Attempts + 1},
				logger.Field{Key: "error", Value: sendErr},
			)
			continue
		}
		if err = r.repo.MarkPublished(ctx, msg.ID); err != nil {
			return err
		}
		r.logger.Debug("outbox 事件投递成功",
			logger.Field{Key: "outbox_id", Value: msg.ID},
			logger.Field{Key: "topic", Value: msg.Topic},
		)
	}
	return nil
}

func (r *OutboxRelay) Run(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := r.DispatchBatch(ctx); err != nil {
			r.logger.Error("outbox relay 调度失败",
				logger.Field{Key: "error", Value: err},
			)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (r *OutboxRelay) Close() error {
	if r == nil || r.producer == nil {
		return nil
	}
	return r.producer.Close()
}
