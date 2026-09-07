package worker

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type hotRankRecorderStub struct {
	recordFn func(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error)
}

func (s *hotRankRecorderStub) Record(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error) {
	return s.recordFn(ctx, eventID, eventType, noteID, occurredAt)
}

func TestHotRankHandler_HandlesAllTopics(t *testing.T) {
	t.Parallel()
	for _, topic := range []string{events.TopicNotePublished, events.TopicNoteLiked, events.TopicCommentCreated} {
		t.Run(topic, func(t *testing.T) {
			t.Parallel()
			payload, err := json.Marshal(hotRankEventEnvelope{EventID: "evt-1", Type: topic, NoteID: 7, OccurredAt: 123})
			if err != nil {
				t.Fatal(err)
			}
			h := NewHotRankHandler(&hotRankRecorderStub{recordFn: func(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error) {
				if eventID != "evt-1" || eventType != topic || noteID != 7 || occurredAt != 123 {
					t.Fatalf("event=%s type=%s note=%d occurred=%d", eventID, eventType, noteID, occurredAt)
				}
				return true, nil
			}}, logger.NewNoOpLogger())
			if err = h.Handle(context.Background(), &sarama.ConsumerMessage{Topic: topic, Value: payload}); err != nil {
				t.Fatalf("handle: %v", err)
			}
		})
	}
}

func TestHotRankHandler_RejectsTopicMismatch(t *testing.T) {
	t.Parallel()
	payload, _ := json.Marshal(hotRankEventEnvelope{EventID: "evt", Type: events.TopicNoteLiked, NoteID: 7, OccurredAt: 123})
	h := NewHotRankHandler(&hotRankRecorderStub{recordFn: func(context.Context, string, string, int64, int64) (bool, error) {
		t.Fatal("recorder must not be called")
		return false, nil
	}}, logger.NewNoOpLogger())
	if err := h.Handle(context.Background(), &sarama.ConsumerMessage{Topic: events.TopicCommentCreated, Value: payload}); err == nil {
		t.Fatal("want topic mismatch error")
	}
}

func TestChainHandlers_OrderAndFailure(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("second failed")
	var calls []int
	handler := ChainHandlers(
		func(context.Context, *sarama.ConsumerMessage) error { calls = append(calls, 1); return nil },
		func(context.Context, *sarama.ConsumerMessage) error { calls = append(calls, 2); return wantErr },
		func(context.Context, *sarama.ConsumerMessage) error { calls = append(calls, 3); return nil },
	)
	err := handler(context.Background(), &sarama.ConsumerMessage{})
	if !errors.Is(err, wantErr) || !reflect.DeepEqual(calls, []int{1, 2}) {
		t.Fatalf("err=%v calls=%v", err, calls)
	}
}
