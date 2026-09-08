package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var ErrNotificationNotFound = dao.ErrNotificationNotFound

type NotificationRepository interface {
	CreateIfAbsent(ctx context.Context, notification domain.Notification) (bool, error)
	ListByReceiver(ctx context.Context, receiverID int64, cursor *domain.NotificationCursor, limit int) ([]domain.Notification, error)
	MarkRead(ctx context.Context, receiverID, notificationID int64) error
	FindNoteAuthorID(ctx context.Context, noteID int64) (int64, error)
}

type NotificationRepositoryImpl struct {
	notifications dao.NotificationDAO
	notes         dao.NoteDAO
}

func NewNotificationRepositoryImpl(notifications dao.NotificationDAO, notes dao.NoteDAO) *NotificationRepositoryImpl {
	return &NotificationRepositoryImpl{notifications: notifications, notes: notes}
}

func (r *NotificationRepositoryImpl) CreateIfAbsent(ctx context.Context, notification domain.Notification) (bool, error) {
	return r.notifications.InsertIfAbsent(ctx, dao.NotificationOfDB{
		EventId:    notification.EventID,
		ReceiverId: notification.ReceiverID,
		ActorId:    notification.ActorID,
		Type:       notification.Type,
		BizId:      notification.BizID,
		IsRead:     notification.IsRead,
		CreatedAt:  notification.CreatedAt,
	})
}

func (r *NotificationRepositoryImpl) ListByReceiver(ctx context.Context, receiverID int64, cursor *domain.NotificationCursor, limit int) ([]domain.Notification, error) {
	var daoCursor *dao.NotificationCursor
	if cursor != nil {
		daoCursor = &dao.NotificationCursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID}
	}
	rows, err := r.notifications.ListByReceiver(ctx, receiverID, daoCursor, limit)
	if err != nil {
		return nil, err
	}
	result := make([]domain.Notification, 0, len(rows))
	for _, row := range rows {
		result = append(result, toDomainNotification(row))
	}
	return result, nil
}

func (r *NotificationRepositoryImpl) MarkRead(ctx context.Context, receiverID, notificationID int64) error {
	return r.notifications.MarkRead(ctx, receiverID, notificationID)
}

func (r *NotificationRepositoryImpl) FindNoteAuthorID(ctx context.Context, noteID int64) (int64, error) {
	note, err := r.notes.FindByID(ctx, noteID)
	if err != nil {
		return 0, err
	}
	return note.AuthorId, nil
}

func toDomainNotification(row dao.NotificationOfDB) domain.Notification {
	return domain.Notification{
		ID:         row.Id,
		EventID:    row.EventId,
		ReceiverID: row.ReceiverId,
		ActorID:    row.ActorId,
		Type:       row.Type,
		BizID:      row.BizId,
		IsRead:     row.IsRead,
		CreatedAt:  row.CreatedAt,
	}
}
