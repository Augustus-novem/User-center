package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/internal/worker"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type UserRegisteredHandler struct {
	welcomeMessageRepo repository.WelcomeMessageRepository
	deduper            worker.Deduplicator
	logger             logger.Logger
}

func NewUserRegisteredHandler(welcomeMessageRepo repository.WelcomeMessageRepository, deduper worker.Deduplicator, l logger.Logger) *UserRegisteredHandler {
	return &UserRegisteredHandler{
		welcomeMessageRepo: welcomeMessageRepo,
		deduper:            deduper,
		logger:             l,
	}
}

func (h *UserRegisteredHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
	var evt events.UserRegisteredEvent
	if err = json.Unmarshal(msg.Value, &evt); err != nil {
		return fmt.Errorf("unmarshal user registered event: %w", err)
	}
	started, err := h.deduper.TryBegin(ctx, evt.EventID)
	if err != nil {
		return err
	}
	if !started {
		h.logger.Info("注册事件重复消费或正在处理中，已跳过欢迎消息发送",
			logger.Field{Key: "event_id", Value: evt.EventID},
			logger.Field{Key: "user_id", Value: evt.UserID},
		)
		return nil
	}
	defer func() {
		if err != nil {
			_ = h.deduper.ClearInFlight(ctx, evt.EventID)
		}
	}()
	created, err := h.welcomeMessageRepo.SaveIfAbsent(ctx, repository.WelcomeMessage{
		UserID:     evt.UserID,
		Email:      evt.Email,
		Title:      "欢迎加入 user-center",
		Content:    fmt.Sprintf("欢迎注册 user-center，已为你发放 %d 欢迎积分。", repository.DefaultWelcomePoints),
		CreatedAt:  time.Now().UnixMilli(),
		OccurredAt: evt.OccurredAt,
	})
	if err != nil {
		return err
	}
	if err = h.deduper.MarkDone(ctx, evt.EventID); err != nil {
		return err
	}
	if created {
		h.logger.Info("注册事件消费成功：已写入欢迎消息",
			logger.Field{Key: "event_id", Value: evt.EventID},
			logger.Field{Key: "user_id", Value: evt.UserID},
			logger.Field{Key: "email", Value: evt.Email},
		)
		return nil
	}
	h.logger.Info("注册事件幂等命中：欢迎消息已存在，无需重复写入",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "user_id", Value: evt.UserID},
	)
	return nil
}
