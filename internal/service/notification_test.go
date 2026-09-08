package service

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/domain"
)

type notificationRepoStub struct {
	createFn     func(context.Context, domain.Notification) (bool, error)
	listFn       func(context.Context, int64, *domain.NotificationCursor, int) ([]domain.Notification, error)
	markReadFn   func(context.Context, int64, int64) error
	noteAuthorFn func(context.Context, int64) (int64, error)
}

func (s *notificationRepoStub) CreateIfAbsent(ctx context.Context, notification domain.Notification) (bool, error) {
	return s.createFn(ctx, notification)
}

func (s *notificationRepoStub) ListByReceiver(ctx context.Context, receiverID int64, cursor *domain.NotificationCursor, limit int) ([]domain.Notification, error) {
	return s.listFn(ctx, receiverID, cursor, limit)
}

func (s *notificationRepoStub) MarkRead(ctx context.Context, receiverID, notificationID int64) error {
	return s.markReadFn(ctx, receiverID, notificationID)
}

func (s *notificationRepoStub) FindNoteAuthorID(ctx context.Context, noteID int64) (int64, error) {
	return s.noteAuthorFn(ctx, noteID)
}

func TestNotificationService_DuplicateEventIsIdempotent(t *testing.T) {
	t.Parallel()
	repo := &notificationRepoStub{createFn: func(context.Context, domain.Notification) (bool, error) {
		return false, nil
	}}
	created, err := NewNotificationServiceImpl(repo).CreateFollow(context.Background(), "evt-1", 1, 2, 100)
	if err != nil || created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestNotificationService_SelfActionIsIgnored(t *testing.T) {
	t.Parallel()
	called := false
	repo := &notificationRepoStub{
		createFn: func(context.Context, domain.Notification) (bool, error) {
			called = true
			return true, nil
		},
		noteAuthorFn: func(context.Context, int64) (int64, error) { return 7, nil },
	}
	created, err := NewNotificationServiceImpl(repo).CreateLike(context.Background(), "evt-2", 7, 9, 100)
	if err != nil || created || called {
		t.Fatalf("created=%v called=%v err=%v", created, called, err)
	}
}

func TestNotificationService_MapsCommentTarget(t *testing.T) {
	t.Parallel()
	repo := &notificationRepoStub{
		noteAuthorFn: func(ctx context.Context, noteID int64) (int64, error) {
			if noteID != 9 {
				t.Fatalf("noteID=%d", noteID)
			}
			return 8, nil
		},
		createFn: func(ctx context.Context, n domain.Notification) (bool, error) {
			if n.EventID != "evt-3" || n.ReceiverID != 8 || n.ActorID != 7 || n.Type != domain.NotificationTypeComment || n.BizID != 10 || n.CreatedAt != 100 {
				t.Fatalf("unexpected notification: %+v", n)
			}
			return true, nil
		},
	}
	created, err := NewNotificationServiceImpl(repo).CreateComment(context.Background(), "evt-3", 7, 9, 10, 100)
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestNotificationService_ReadIsIdempotent(t *testing.T) {
	t.Parallel()
	calls := 0
	repo := &notificationRepoStub{markReadFn: func(context.Context, int64, int64) error {
		calls++
		return nil
	}}
	svc := NewNotificationServiceImpl(repo)
	if err := svc.MarkRead(context.Background(), 4, 3); err != nil {
		t.Fatal(err)
	}
	if err := svc.MarkRead(context.Background(), 4, 3); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("calls=%d", calls)
	}
}

func TestNotificationService_StableCursorPagination(t *testing.T) {
	t.Parallel()
	repo := &notificationRepoStub{listFn: func(ctx context.Context, receiverID int64, cursor *domain.NotificationCursor, limit int) ([]domain.Notification, error) {
		if receiverID != 4 || limit != 2 {
			t.Fatalf("receiver=%d limit=%d", receiverID, limit)
		}
		if cursor == nil {
			return []domain.Notification{{ID: 3, CreatedAt: 100}, {ID: 2, CreatedAt: 100}}, nil
		}
		if cursor.CreatedAt != 100 || cursor.ID != 3 {
			t.Fatalf("cursor=%+v", cursor)
		}
		return []domain.Notification{{ID: 2, CreatedAt: 100}}, nil
	}}
	svc := NewNotificationServiceImpl(repo)
	page1, err := svc.List(context.Background(), 4, "", 1)
	if err != nil || len(page1.Items) != 1 || page1.Items[0].ID != 3 || !page1.HasMore || page1.NextCursor != "100_3" {
		t.Fatalf("page1=%+v err=%v", page1, err)
	}
	page2, err := svc.List(context.Background(), 4, page1.NextCursor, 1)
	if err != nil || len(page2.Items) != 1 || page2.Items[0].ID != 2 {
		t.Fatalf("page2=%+v err=%v", page2, err)
	}
}

func TestNotificationService_InvalidCursor(t *testing.T) {
	t.Parallel()
	svc := NewNotificationServiceImpl(&notificationRepoStub{})
	_, err := svc.List(context.Background(), 1, "bad", 20)
	if !errors.Is(err, ErrInvalidNotificationCursor) {
		t.Fatalf("want invalid cursor, got %v", err)
	}
}
