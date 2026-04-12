package events

import (
	"context"
	"encoding/json"
	"strconv"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, key string, value any) error
	IsEnabled() bool
}

type NopPublisher struct{}

func (n NopPublisher) Publish(ctx context.Context, topic string, key string, value any) error {
	return nil
}

func (n NopPublisher) IsEnabled() bool {
	return false
}

type OutboxPublisher struct {
	repo   repository.EventOutboxRepository
	logger logger.Logger
}

func NewOutboxPublisher(repo repository.EventOutboxRepository, logger logger.Logger) *OutboxPublisher {
	return &OutboxPublisher{repo: repo, logger: logger}
}

func (p *OutboxPublisher) Publish(ctx context.Context, topic string, key string, value any) error {
	bs, err := json.Marshal(value)
	if err != nil {
		return err
	}
	msg, err := p.repo.Add(ctx, topic, key, bs)
	if err != nil {
		return err
	}
	p.logger.Debug("事件已写入 outbox",
		logger.Field{Key: "topic", Value: topic},
		logger.Field{Key: "key", Value: key},
		logger.Field{Key: "outbox_id", Value: msg.ID},
	)
	return nil
}

func (p *OutboxPublisher) IsEnabled() bool {
	return p != nil && p.repo != nil
}

func UserIDKey(userID int64) string {
	return strconv.FormatInt(userID, 10)
}
