package worker

import (
	"context"
	"encoding/json"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type SearchEventIndexer interface {
	IndexPublished(ctx context.Context, noteID int64) error
	Delete(ctx context.Context, noteID int64) error
}

type SearchIndexHandler struct {
	indexer SearchEventIndexer
	logger  logger.Logger
}

func NewSearchIndexHandler(indexer SearchEventIndexer, l logger.Logger) *SearchIndexHandler {
	return &SearchIndexHandler{indexer: indexer, logger: l}
}

func (h *SearchIndexHandler) HandlePublished(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt events.NotePublishedEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return Permanentf("unmarshal note published event: %w", err)
	}
	if evt.EventID == "" || evt.Type != events.TopicNotePublished || evt.NoteID <= 0 || evt.AuthorID <= 0 || evt.OccurredAt <= 0 {
		return invalidSearchEvent(events.TopicNotePublished)
	}
	if err := h.indexer.IndexPublished(ctx, evt.NoteID); err != nil {
		return err
	}
	h.logger.Info("搜索索引已处理 note.published",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "note_id", Value: evt.NoteID},
	)
	return nil
}

func (h *SearchIndexHandler) HandleDeleted(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt events.NoteDeletedEvent
	if err := json.Unmarshal(msg.Value, &evt); err != nil {
		return Permanentf("unmarshal note deleted event: %w", err)
	}
	if evt.EventID == "" || evt.Type != events.TopicNoteDeleted || evt.NoteID <= 0 || evt.AuthorID <= 0 || evt.OccurredAt <= 0 {
		return invalidSearchEvent(events.TopicNoteDeleted)
	}
	if err := h.indexer.Delete(ctx, evt.NoteID); err != nil {
		return err
	}
	h.logger.Info("搜索索引已处理 note.deleted",
		logger.Field{Key: "event_id", Value: evt.EventID},
		logger.Field{Key: "note_id", Value: evt.NoteID},
	)
	return nil
}

func invalidSearchEvent(topic string) error {
	return Permanentf("invalid %s event", topic)
}
