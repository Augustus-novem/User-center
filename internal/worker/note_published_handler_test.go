package worker

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
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

	t.Run("busy event returns explicit transient error", func(t *testing.T) {
		t.Parallel()
		fanout := &stubFanout{}
		deduper := &stubDeduplicator{state: DeduplicationBusy}
		h := NewNotePublishedHandler(fanout, deduper, logger.NewNoOpLogger())
		if err := h.Handle(context.Background(), msg); !errors.Is(err, ErrMessageInFlight) {
			t.Fatalf("want ErrMessageInFlight, got %v", err)
		}
		if fanout.calls != 0 || deduper.marks != 0 || deduper.clears != 0 {
			t.Fatalf("BUSY must have no side effect: fanout=%d marks=%d clears=%d", fanout.calls, deduper.marks, deduper.clears)
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

	t.Run("mark done failure is returned", func(t *testing.T) {
		t.Parallel()
		want := errors.New("redis mark failed")
		deduper := &stubDeduplicator{markErr: want}
		h := NewNotePublishedHandler(&stubFanout{}, deduper, logger.NewNoOpLogger())
		if err := h.Handle(context.Background(), msg); !errors.Is(err, want) {
			t.Fatalf("want mark error, got %v", err)
		}
		if deduper.marks != 1 {
			t.Fatalf("marks=%d", deduper.marks)
		}
	})

	t.Run("cleanup failure is returned and redelivery can retry", func(t *testing.T) {
		t.Parallel()
		fanout := &stubFanout{failNth: 1}
		deduper := &stubDeduplicator{clearErr: errors.New("redis cleanup failed")}
		h := NewNotePublishedHandler(fanout, deduper, logger.NewNoOpLogger())
		firstErr := h.Handle(context.Background(), msg)
		if firstErr == nil || !strings.Contains(firstErr.Error(), "cleanup failed lease") {
			t.Fatalf("want joined cleanup error, got %v", firstErr)
		}
		deduper.clearErr = nil
		deduper.state = DeduplicationBusy
		if err := h.Handle(context.Background(), msg); !errors.Is(err, ErrMessageInFlight) {
			t.Fatalf("redelivery before lease expiry should be BUSY, got %v", err)
		}
		if fanout.calls != 1 {
			t.Fatalf("BUSY redelivery must not rerun business, calls=%d", fanout.calls)
		}
		deduper.state = DeduplicationAcquired
		if err := h.Handle(context.Background(), msg); err != nil {
			t.Fatalf("redelivery after lease expiry: %v", err)
		}
		if fanout.calls != 2 || deduper.marks != 1 {
			t.Fatalf("fanout=%d marks=%d", fanout.calls, deduper.marks)
		}
	})
}
