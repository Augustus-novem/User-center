package worker

import (
	"context"
	"encoding/json"
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
		return Permanentf("unmarshal note published event: %w", err)
	}
	if evt.EventID == "" || evt.Type != events.TopicNotePublished || evt.NoteID <= 0 || evt.AuthorID <= 0 || evt.OccurredAt <= 0 {
		return Permanentf("invalid %s event", events.TopicNotePublished)
	}
	state, err := RunDeduplicated(ctx, h.deduper, evt.EventID, func(workCtx context.Context) error {
		return h.fanout.FanoutPublished(workCtx, evt.NoteID, evt.AuthorID)
	})
	if err != nil {
		return err
	}
	if state == DeduplicationDone {
		h.logger.Info("note.published 已完成，跳过重复扇出",
			logger.Field{Key: "event_id", Value: evt.EventID},
			logger.Field{Key: "note_id", Value: evt.NoteID},
			logger.Field{Key: "user_id", Value: evt.AuthorID},
		)
		return nil
	}
	h.logger.Info("note.published 扇出完成",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "note_id", Value: evt.NoteID},
		logger.Field{Key: "user_id", Value: evt.AuthorID},
	)
	return nil
}
