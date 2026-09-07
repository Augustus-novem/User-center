package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
	"user-center/internal/events"
	"user-center/internal/repository"
)

type hotRankRepoStub struct {
	recordFn func(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error)
	pageFn   func(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]repository.HotRankItem, error)
}

func (s *hotRankRepoStub) Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error) {
	return s.recordFn(ctx, eventID, noteID, occurredAt, weight)
}

func (s *hotRankRepoStub) SnapshotPage(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]repository.HotRankItem, error) {
	if s.pageFn == nil {
		return nil, nil
	}
	return s.pageFn(ctx, snapshot, offset, limit, create)
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
			svc := NewHotRankServiceImpl(repo, HotRankWeights{Publish: 10, Like: 3, Comment: 5})
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
	}}, HotRankWeights{Publish: 10, Like: 3, Comment: 5})
	_, err := svc.Record(context.Background(), "", events.TopicNoteLiked, 1, 1)
	if !errors.Is(err, ErrInvalidHotRankEvent) {
		t.Fatalf("want ErrInvalidHotRankEvent, got %v", err)
	}
}

func TestHotRankService_ListKeepsSnapshotAcrossPages(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 7, 20, 10, 45, 0, time.Local)
	var calls []struct {
		snapshot time.Time
		offset   int64
		limit    int64
		create   bool
	}
	repo := &hotRankRepoStub{pageFn: func(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]repository.HotRankItem, error) {
		calls = append(calls, struct {
			snapshot time.Time
			offset   int64
			limit    int64
			create   bool
		}{snapshot, offset, limit, create})
		if create {
			return []repository.HotRankItem{{NoteID: 3, Score: 30}, {NoteID: 2, Score: 20}, {NoteID: 1, Score: 10}}, nil
		}
		return []repository.HotRankItem{{NoteID: 1, Score: 10}}, nil
	}}
	svc := NewHotRankServiceImpl(repo, HotRankWeights{})
	svc.now = func() time.Time { return now }

	first, err := svc.List(context.Background(), "", 2)
	if err != nil || !first.HasMore || len(first.Items) != 2 || first.Items[0].Rank != 1 || first.NextCursor == "" {
		t.Fatalf("first=%+v err=%v", first, err)
	}
	now = now.Add(2 * time.Minute)
	second, err := svc.List(context.Background(), first.NextCursor, 2)
	if err != nil || second.HasMore || len(second.Items) != 1 || second.Items[0].Rank != 3 {
		t.Fatalf("second=%+v err=%v", second, err)
	}
	if len(calls) != 2 || !calls[0].create || calls[1].create || calls[1].offset != 2 || !calls[0].snapshot.Equal(calls[1].snapshot) {
		t.Fatalf("calls=%+v", calls)
	}
}

func TestHotRankService_ListMapsExpiredSnapshot(t *testing.T) {
	t.Parallel()
	repo := &hotRankRepoStub{pageFn: func(context.Context, time.Time, int64, int64, bool) ([]repository.HotRankItem, error) {
		return nil, repository.ErrHotRankSnapshotExpired
	}}
	svc := NewHotRankServiceImpl(repo, HotRankWeights{})
	svc.now = func() time.Time { return time.Date(2026, 9, 7, 20, 10, 0, 0, time.Local) }
	_, err := svc.List(context.Background(), fmt.Sprintf("%d_2", svc.now().Unix()/60), 10)
	if !errors.Is(err, ErrInvalidHotRankCursor) {
		t.Fatalf("want ErrInvalidHotRankCursor, got %v", err)
	}
}
