package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

const NotePublishedDeduperNamespace = "worker:note_published"

type PublishedNoteFanout interface {
	FanoutPublished(ctx context.Context, noteID, authorID int64) error
}

type NotePublishedHandler struct {
	fanout  PublishedNoteFanout
	deduper Deduplicator
	logger  logger.Logger
}

func NewNotePublishedHandler(fanout PublishedNoteFanout, deduper Deduplicator, l logger.Logger) *NotePublishedHandler {
	if l == nil {
		l = logger.NewNoOpLogger()
	}
	return &NotePublishedHandler{
		fanout:  fanout,
		deduper: deduper,
		logger:  l,
	}
}

func (h *NotePublishedHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) (err error) {
	var evt events.NotePublishedEvent
	if err = json.Unmarshal(msg.Value, &evt); err != nil {
		return fmt.Errorf("unmarshal note published event: %w", err)
	}
	started, err := h.deduper.TryBegin(ctx, evt.EventID)
	if err != nil {
		return err
	}
	if !started {
		h.logger.Info("note.published 重复消费或正在处理中，已跳过扇出",
			logger.Field{Key: "event_id", Value: evt.EventID},
			logger.Field{Key: "note_id", Value: evt.NoteID},
			logger.Field{Key: "user_id", Value: evt.AuthorID},
		)
		return nil
	}
	defer func() {
		if err != nil {
			_ = h.deduper.ClearInFlight(ctx, evt.EventID)
		}
	}()
	if err = h.fanout.FanoutPublished(ctx, evt.NoteID, evt.AuthorID); err != nil {
		return err
	}
	if err = h.deduper.MarkDone(ctx, evt.EventID); err != nil {
		return err
	}
	h.logger.Info("note.published 扇出完成",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "note_id", Value: evt.NoteID},
		logger.Field{Key: "user_id", Value: evt.AuthorID},
	)
	return nil
}
