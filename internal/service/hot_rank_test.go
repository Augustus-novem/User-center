package service

import (
	"context"
	"errors"
	"testing"
	"time"
	"user-center/internal/config"
	"user-center/internal/events"
)

type hotRankRepoStub struct {
	recordFn func(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error)
}

func (s *hotRankRepoStub) Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error) {
	return s.recordFn(ctx, eventID, noteID, occurredAt, weight)
}

func TestHotRankService_RecordUsesConfiguredWeights(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		topic string
		want  int64
	}{{events.TopicNotePublished, 10}, {events.TopicNoteLiked, 3}, {events.TopicCommentCreated, 5}} {
		t.Run(tc.topic, func(t *testing.T) {
			t.Parallel()
			repo := &hotRankRepoStub{recordFn: func(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error) {
				if eventID != "evt" || noteID != 9 || occurredAt.UnixMilli() != 1234 || weight != tc.want {
					t.Fatalf("event=%s note=%d occurred=%d weight=%d", eventID, noteID, occurredAt.UnixMilli(), weight)
				}
				return true, nil
			}}
			svc := NewHotRankServiceImpl(repo, config.HotRankConfig{PublishWeight: 10, LikeWeight: 3, CommentWeight: 5})
			applied, err := svc.Record(context.Background(), "evt", tc.topic, 9, 1234)
			if err != nil || !applied {
				t.Fatalf("applied=%v err=%v", applied, err)
			}
		})
	}
}

func TestHotRankService_RecordRejectsInvalidEvent(t *testing.T) {
	t.Parallel()
	svc := NewHotRankServiceImpl(&hotRankRepoStub{recordFn: func(context.Context, string, int64, time.Time, int64) (bool, error) {
		t.Fatal("repository must not be called")
		return false, nil
	}}, config.HotRankConfig{PublishWeight: 10, LikeWeight: 3, CommentWeight: 5})
	_, err := svc.Record(context.Background(), "", events.TopicNoteLiked, 1, 1)
	if !errors.Is(err, ErrInvalidHotRankEvent) {
		t.Fatalf("want ErrInvalidHotRankEvent, got %v", err)
	}
}
