package notification

import (
	"context"
	"encoding/json"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type notificationServiceStub struct {
	followFn  func(context.Context, string, int64, int64, int64) (bool, error)
	likeFn    func(context.Context, string, int64, int64, int64) (bool, error)
	commentFn func(context.Context, string, int64, int64, int64, int64) (bool, error)
}

func (s *notificationServiceStub) CreateFollow(ctx context.Context, eventID string, followerID, followeeID, occurredAt int64) (bool, error) {
	return s.followFn(ctx, eventID, followerID, followeeID, occurredAt)
}

func (s *notificationServiceStub) CreateLike(ctx context.Context, eventID string, actorID, noteID, occurredAt int64) (bool, error) {
	return s.likeFn(ctx, eventID, actorID, noteID, occurredAt)
}

func (s *notificationServiceStub) CreateComment(ctx context.Context, eventID string, actorID, noteID, commentID, occurredAt int64) (bool, error) {
	return s.commentFn(ctx, eventID, actorID, noteID, commentID, occurredAt)
}

func (s *notificationServiceStub) List(context.Context, int64, string, int) (domain.NotificationPage, error) {
	return domain.NotificationPage{}, nil
}

func (s *notificationServiceStub) MarkRead(context.Context, int64, int64) error { return nil }

func TestCommunityHandler_DispatchesEvents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		event  any
		handle func(*CommunityHandler, context.Context, *sarama.ConsumerMessage) error
		stub   *notificationServiceStub
	}{
		{
			name:  "followed",
			event: events.UserFollowedEvent{EventID: "f", Type: events.TopicUserFollowed, FollowerID: 1, FolloweeID: 2, OccurredAt: 3},
			handle: func(h *CommunityHandler, ctx context.Context, msg *sarama.ConsumerMessage) error {
				return h.HandleUserFollowed(ctx, msg)
			},
			stub: &notificationServiceStub{followFn: func(ctx context.Context, eventID string, actor, receiver, at int64) (bool, error) {
				if eventID != "f" || actor != 1 || receiver != 2 || at != 3 {
					t.Fatalf("unexpected follow args")
				}
				return true, nil
			}},
		},
		{
			name:  "liked",
			event: events.NoteLikedEvent{EventID: "l", Type: events.TopicNoteLiked, NoteID: 4, UserID: 5, OccurredAt: 6},
			handle: func(h *CommunityHandler, ctx context.Context, msg *sarama.ConsumerMessage) error {
				return h.HandleNoteLiked(ctx, msg)
			},
			stub: &notificationServiceStub{likeFn: func(ctx context.Context, eventID string, actor, noteID, at int64) (bool, error) {
				if eventID != "l" || actor != 5 || noteID != 4 || at != 6 {
					t.Fatalf("unexpected like args")
				}
				return true, nil
			}},
		},
		{
			name:  "commented",
			event: events.CommentCreatedEvent{EventID: "c", Type: events.TopicCommentCreated, CommentID: 7, NoteID: 8, UserID: 9, OccurredAt: 10},
			handle: func(h *CommunityHandler, ctx context.Context, msg *sarama.ConsumerMessage) error {
				return h.HandleCommentCreated(ctx, msg)
			},
			stub: &notificationServiceStub{commentFn: func(ctx context.Context, eventID string, actor, noteID, commentID, at int64) (bool, error) {
				if eventID != "c" || actor != 9 || noteID != 8 || commentID != 7 || at != 10 {
					t.Fatalf("unexpected comment args")
				}
				return true, nil
			}},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			body, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatal(err)
			}
			h := NewCommunityHandler(tt.stub, logger.NewNoOpLogger())
			if err = tt.handle(h, context.Background(), &sarama.ConsumerMessage{Value: body}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommunityHandler_RejectsWrongEventType(t *testing.T) {
	t.Parallel()
	body, _ := json.Marshal(events.NoteLikedEvent{EventID: "x", Type: events.TopicCommentCreated})
	h := NewCommunityHandler(&notificationServiceStub{}, logger.NewNoOpLogger())
	if err := h.HandleNoteLiked(context.Background(), &sarama.ConsumerMessage{Value: body}); err == nil {
		t.Fatal("expected type validation error")
	}
}
