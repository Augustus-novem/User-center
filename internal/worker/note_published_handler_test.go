package worker

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type stubFanout struct {
	calls   int
	noteID  int64
	author  int64
	err     error
	failNth int
}

func (s *stubFanout) FanoutPublished(ctx context.Context, noteID, authorID int64) error {
	s.calls++
	s.noteID = noteID
	s.author = authorID
	if s.failNth > 0 && s.calls == s.failNth {
		return errors.New("fanout batch failed")
	}
	return s.err
}

func TestNotePublishedHandler_Handle(t *testing.T) {
	t.Parallel()

	evt := events.NewNotePublishedEvent(42, 9)
	payload, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	msg := &sarama.ConsumerMessage{Value: payload}

	t.Run("success marks done", func(t *testing.T) {
		t.Parallel()
		fanout := &stubFanout{}
		deduper := &stubDeduplicator{}
		h := NewNotePublishedHandler(fanout, deduper, logger.NewNoOpLogger())
		if err := h.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if fanout.calls != 1 || fanout.noteID != 42 || fanout.author != 9 {
			t.Fatalf("unexpected fanout: %+v", fanout)
		}
		if deduper.marks != 1 || deduper.clears != 0 {
			t.Fatalf("deduper marks=%d clears=%d", deduper.marks, deduper.clears)
		}
	})

	t.Run("duplicate event skips fanout", func(t *testing.T) {
		t.Parallel()
		fanout := &stubFanout{}
		deduper := &stubDeduplicator{started: true}
		h := NewNotePublishedHandler(fanout, deduper, logger.NewNoOpLogger())
		if err := h.Handle(context.Background(), msg); err != nil {
			t.Fatalf("Handle: %v", err)
		}
		if fanout.calls != 0 {
			t.Fatalf("duplicate must not fanout, calls=%d", fanout.calls)
		}
		if deduper.marks != 0 {
			t.Fatalf("duplicate must not MarkDone, marks=%d", deduper.marks)
		}
	})

	t.Run("partial fanout failure clears in-flight", func(t *testing.T) {
		t.Parallel()
		fanout := &stubFanout{failNth: 1}
		deduper := &stubDeduplicator{}
		h := NewNotePublishedHandler(fanout, deduper, logger.NewNoOpLogger())
		if err := h.Handle(context.Background(), msg); err == nil {
			t.Fatal("want fanout error")
		}
		if deduper.marks != 0 {
			t.Fatal("failed handle must not MarkDone")
		}
		if deduper.clears != 1 {
			t.Fatalf("want ClearInFlight, clears=%d", deduper.clears)
		}
	})

	t.Run("retry after failure succeeds", func(t *testing.T) {
		t.Parallel()
		fanout := &stubFanout{failNth: 1}
		deduper := &stubDeduplicator{}
		h := NewNotePublishedHandler(fanout, deduper, logger.NewNoOpLogger())
		if err := h.Handle(context.Background(), msg); err == nil {
			t.Fatal("want first handle error")
		}
		if err := h.Handle(context.Background(), msg); err != nil {
			t.Fatalf("retry: %v", err)
		}
		if fanout.calls != 2 {
			t.Fatalf("want two fanout attempts, got %d", fanout.calls)
		}
		if deduper.marks != 1 {
			t.Fatalf("retry must MarkDone once, marks=%d", deduper.marks)
		}
	})
}
