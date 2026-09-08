package worker

import (
	"context"
	"encoding/json"
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
		return Permanentf("unmarshal user registered event: %w", err)
	}
	if evt.EventID == "" || evt.Type != events.TopicUserRegistered || evt.UserID <= 0 || evt.OccurredAt <= 0 {
		return Permanentf("invalid %s event", events.TopicUserRegistered)
	}
	state, err := RunDeduplicated(ctx, h.deduper, evt.EventID, func(workCtx context.Context) error {
		return h.pointRepo.AddWelcomePoints(workCtx, evt.UserID, repository.DefaultWelcomePoints)
	})
	if err != nil {
		return err
	}
	if state == DeduplicationDone {
		h.logger.Info("注册事件已完成，跳过重复欢迎积分初始化",
			logger.Field{Key: "event_id", Value: evt.EventID},
			logger.Field{Key: "user_id", Value: evt.UserID},
		)
		return nil
	}
	h.logger.Info("注册事件消费成功：已初始化欢迎积分",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "user_id", Value: evt.UserID},
		logger.Field{Key: "points", Value: repository.DefaultWelcomePoints},
	)
	return nil
}
