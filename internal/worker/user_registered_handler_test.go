package worker

import (
	"context"
	"encoding/json"
	"testing"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type stubPointRepository struct {
	userID int64
	points int
	calls  int
}

func (s *stubPointRepository) AddSignInPoints(ctx context.Context, userID int64, signInAt int64, points int) error {
	return nil
}

func (s *stubPointRepository) AddWelcomePoints(ctx context.Context, userID int64, points int) error {
	s.userID = userID
	s.points = points
	s.calls++
	return nil
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
	repo := &stubPointRepository{}
	deduper := &stubDeduplicator{}
	h := NewUserRegisteredHandler(repo, deduper, logger.NewNoOpLogger())

	evt := events.NewUserRegisteredEvent(123, "demo@example.com")
	bs, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}

	err = h.Handle(context.Background(), &sarama.ConsumerMessage{Value: bs})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if repo.calls != 1 {
		t.Fatalf("expected AddWelcomePoints to be called once, got %d", repo.calls)
	}
	if repo.userID != evt.UserID {
		t.Fatalf("expected userID %d, got %d", evt.UserID, repo.userID)
	}
	if repo.points != repository.DefaultWelcomePoints {
		t.Fatalf("expected points %d, got %d", repository.DefaultWelcomePoints, repo.points)
	}
	if deduper.marks != 1 {
		t.Fatalf("expected MarkDone to be called once, got %d", deduper.marks)
	}
}
