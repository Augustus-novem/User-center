package worker

import (
	"context"
	"encoding/json"
	"testing"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type searchEventIndexerStub struct {
	published []int64
	deleted   []int64
}

func (s *searchEventIndexerStub) IndexPublished(_ context.Context, noteID int64) error {
	s.published = append(s.published, noteID)
	return nil
}
func (s *searchEventIndexerStub) Delete(_ context.Context, noteID int64) error {
	s.deleted = append(s.deleted, noteID)
	return nil
}

func TestSearchIndexHandler(t *testing.T) {
	t.Parallel()
	indexer := &searchEventIndexerStub{}
	h := NewSearchIndexHandler(indexer, logger.NewNoOpLogger())
	published, _ := json.Marshal(events.NewNotePublishedEvent(4, 2))
	deleted, _ := json.Marshal(events.NewNoteDeletedEvent(4, 2))
	if err := h.HandlePublished(context.Background(), &sarama.ConsumerMessage{Value: published}); err != nil {
		t.Fatal(err)
	}
	if err := h.HandleDeleted(context.Background(), &sarama.ConsumerMessage{Value: deleted}); err != nil {
		t.Fatal(err)
	}
	if len(indexer.published) != 1 || indexer.published[0] != 4 || len(indexer.deleted) != 1 || indexer.deleted[0] != 4 {
		t.Fatalf("published=%v deleted=%v", indexer.published, indexer.deleted)
	}
}
