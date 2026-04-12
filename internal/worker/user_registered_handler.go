package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type UserRegisteredHandler struct {
	pointRepo repository.PointRepository
	deduper   Deduplicator
	logger    logger.Logger
}

func NewUserRegisteredHandler(pointRepo repository.PointRepository, deduper Deduplicator, l logger.Logger) *UserRegisteredHandler {
	return &UserRegisteredHandler{
		pointRepo: pointRepo,
		deduper:   deduper,
		logger:    l,
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
		h.logger.Info("注册事件重复消费或正在处理中，已跳过欢迎积分初始化",
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
	if err = h.pointRepo.AddWelcomePoints(ctx, evt.UserID, repository.DefaultWelcomePoints); err != nil {
		return err
	}
	if err = h.deduper.MarkDone(ctx, evt.EventID); err != nil {
		return err
	}
	h.logger.Info("注册事件消费成功：已初始化欢迎积分",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "user_id", Value: evt.UserID},
		logger.Field{Key: "points", Value: repository.DefaultWelcomePoints},
	)
	return nil
}
