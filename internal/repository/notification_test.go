package repository

import (
	"context"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

type notificationDAOStub struct {
	insertFn   func(context.Context, dao.NotificationOfDB) (bool, error)
	listFn     func(context.Context, int64, *dao.NotificationCursor, int) ([]dao.NotificationOfDB, error)
	markReadFn func(context.Context, int64, int64) error
}

func (s *notificationDAOStub) InsertIfAbsent(ctx context.Context, row dao.NotificationOfDB) (bool, error) {
	return s.insertFn(ctx, row)
}

func (s *notificationDAOStub) ListByReceiver(ctx context.Context, receiverID int64, cursor *dao.NotificationCursor, limit int) ([]dao.NotificationOfDB, error) {
	return s.listFn(ctx, receiverID, cursor, limit)
}

func (s *notificationDAOStub) MarkRead(ctx context.Context, receiverID, notificationID int64) error {
	return s.markReadFn(ctx, receiverID, notificationID)
}

func TestNotificationRepository_CreateMapsFields(t *testing.T) {
	t.Parallel()
	repo := NewNotificationRepositoryImpl(&notificationDAOStub{
		insertFn: func(ctx context.Context, row dao.NotificationOfDB) (bool, error) {
			if row.EventId != "evt-1" || row.ReceiverId != 2 || row.ActorId != 1 || row.Type != domain.NotificationTypeFollow || row.BizId != 1 || row.CreatedAt != 100 {
				t.Fatalf("unexpected row: %+v", row)
			}
			return true, nil
		},
	}, nil)

	created, err := repo.CreateIfAbsent(context.Background(), domain.Notification{
		EventID: "evt-1", ReceiverID: 2, ActorID: 1,
		Type: domain.NotificationTypeFollow, BizID: 1, CreatedAt: 100,
	})
	if err != nil || !created {
		t.Fatalf("created=%v err=%v", created, err)
	}
}

func TestNotificationRepository_ListMapsStableCursor(t *testing.T) {
	t.Parallel()
	repo := NewNotificationRepositoryImpl(&notificationDAOStub{
		listFn: func(ctx context.Context, receiverID int64, cursor *dao.NotificationCursor, limit int) ([]dao.NotificationOfDB, error) {
			if receiverID != 7 || cursor == nil || cursor.CreatedAt != 100 || cursor.ID != 9 || limit != 21 {
				t.Fatalf("unexpected list args receiver=%d cursor=%+v limit=%d", receiverID, cursor, limit)
			}
			return []dao.NotificationOfDB{{
				Id: 8, EventId: "evt-2", ReceiverId: 7, ActorId: 3,
				Type: domain.NotificationTypeLike, BizId: 4, IsRead: true, CreatedAt: 99,
			}}, nil
		},
	}, nil)

	rows, err := repo.ListByReceiver(context.Background(), 7, &domain.NotificationCursor{CreatedAt: 100, ID: 9}, 21)
	if err != nil || len(rows) != 1 || rows[0].ID != 8 || rows[0].EventID != "evt-2" || !rows[0].IsRead {
		t.Fatalf("rows=%+v err=%v", rows, err)
	}
}

func TestNotificationRepository_FindsDeletedNoteAuthor(t *testing.T) {
	t.Parallel()
	repo := NewNotificationRepositoryImpl(nil, &noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			return dao.NoteOfDB{Id: id, AuthorId: 12, Status: domain.NoteStatusDeleted}, nil
		},
	})

	authorID, err := repo.FindNoteAuthorID(context.Background(), 5)
	if err != nil || authorID != 12 {
		t.Fatalf("author=%d err=%v", authorID, err)
	}
}
