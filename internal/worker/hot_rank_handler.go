package worker

import (
	"context"
	"encoding/json"
	"user-center/internal/service"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type hotRankEventEnvelope struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	NoteID     int64  `json:"note_id"`
	OccurredAt int64  `json:"occurred_at"`
}

type HotRankHandler struct {
	recorder service.HotRankRecorder
	logger   logger.Logger
}

func NewHotRankHandler(recorder service.HotRankRecorder, l logger.Logger) *HotRankHandler {
	if l == nil {
		l = logger.NewNoOpLogger()
	}
	return &HotRankHandler{recorder: recorder, logger: l}
}

func (h *HotRankHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt hotRankEventEnvelope
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return Permanentf("unmarshal hot rank event: %w", err)
	}
	if evt.Type != msg.Topic {
		return Permanentf("hot rank event type %q does not match topic %q", evt.Type, msg.Topic)
	}
	if evt.EventID == "" || evt.NoteID <= 0 || evt.OccurredAt <= 0 {
		return Permanentf("invalid hot rank event for topic %q", msg.Topic)
	}
	applied, err := h.recorder.Record(ctx, evt.EventID, evt.Type, evt.NoteID, evt.OccurredAt)
	if err != nil {
		return err
	}
	h.logger.Info("hot ranking event consumed",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "event_type", Value: evt.Type},
		logger.Field{Key: "note_id", Value: evt.NoteID},
		logger.Field{Key: "applied", Value: applied},
	)
	return nil
}

func ChainHandlers(handlers ...MessageHandler) MessageHandler {
	return func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		for _, handler := range handlers {
			if err := handler(ctx, msg); err != nil {
				return err
			}
		}
		return nil
	}
}
