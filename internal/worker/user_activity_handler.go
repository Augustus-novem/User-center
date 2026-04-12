package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type UserActivityHandler struct {
	processor UserActivityProcessor
	logger    logger.Logger
}

func NewUserActivityHandler(processor UserActivityProcessor, l logger.Logger) *UserActivityHandler {
	return &UserActivityHandler{
		processor: processor,
		logger:    l,
	}
}

func (h *UserActivityHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt events.UserActivityEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return fmt.Errorf("unmarshal user activity event: %w", err)
	}
	processed, err := h.processor.ProcessOnce(ctx, evt)
	if err != nil {
		return err
	}
	if !processed {
		h.logger.Info("行为事件重复消费，已跳过",
			logger.Field{Key: "event_id", Value: evt.EventID},
			logger.Field{Key: "user_id", Value: evt.UserID},
		)
		return nil
	}
	h.logger.Info("行为事件消费成功",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "user_id", Value: evt.UserID},
		logger.Field{Key: "action", Value: evt.Action},
	)
	return nil
}
