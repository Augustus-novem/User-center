package notification

import (
	"context"
	"encoding/json"
	"testing"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type stubWelcomeMessageRepository struct {
	msg   repository.WelcomeMessage
	calls int
}

func (s *stubWelcomeMessageRepository) SaveIfAbsent(ctx context.Context, msg repository.WelcomeMessage) (bool, error) {
	s.msg = msg
	s.calls++
	return true, nil
}

type stubDeduplicator struct {
	started bool
	marks   int
	clears  int
	checks  int
}

func (s *stubDeduplicator) TryBegin(ctx context.Context, eventID string) (bool, error) {
	s.checks++
	return !s.started, nil
}

func (s *stubDeduplicator) MarkDone(ctx context.Context, eventID string) error {
	s.marks++
	s.started = true
	return nil
}

func (s *stubDeduplicator) ClearInFlight(ctx context.Context, eventID string) error {
	s.clears++
	return nil
}

func TestUserRegisteredHandler_Handle(t *testing.T) {
	repo := &stubWelcomeMessageRepository{}
	deduper := &stubDeduplicator{}
	h := NewUserRegisteredHandler(repo, deduper, logger.NewNoOpLogger())

	evt := events.NewUserRegisteredEvent(456, "notify@example.com")
	bs, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	err = h.Handle(context.Background(), &sarama.ConsumerMessage{Value: bs})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.calls != 1 {
		t.Fatalf("expected SaveIfAbsent to be called once, got %d", repo.calls)
	}
	if repo.msg.UserID != evt.UserID {
		t.Fatalf("expected userID %d, got %d", evt.UserID, repo.msg.UserID)
	}
	if repo.msg.Email != evt.Email {
		t.Fatalf("expected email %s, got %s", evt.Email, repo.msg.Email)
	}
	if deduper.marks != 1 {
		t.Fatalf("expected MarkDone to be called once, got %d", deduper.marks)
	}
}
