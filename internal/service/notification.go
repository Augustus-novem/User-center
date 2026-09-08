package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"user-center/internal/domain"
	"user-center/internal/repository"
)

var (
	ErrInvalidNotification       = errors.New("通知参数无效")
	ErrInvalidNotificationCursor = errors.New("通知游标无效")
	ErrNotificationNotFound      = repository.ErrNotificationNotFound
)

const (
	defaultNotificationPageSize = 20
	maxNotificationPageSize     = 50
)

type NotificationService interface {
	CreateFollow(ctx context.Context, eventID string, followerID, followeeID, occurredAt int64) (bool, error)
	CreateLike(ctx context.Context, eventID string, actorID, noteID, occurredAt int64) (bool, error)
	CreateComment(ctx context.Context, eventID string, actorID, noteID, commentID, occurredAt int64) (bool, error)
	List(ctx context.Context, receiverID int64, cursor string, limit int) (domain.NotificationPage, error)
	MarkRead(ctx context.Context, receiverID, notificationID int64) error
}

type NotificationServiceImpl struct {
	notifications repository.NotificationRepository
}

func NewNotificationServiceImpl(notifications repository.NotificationRepository) *NotificationServiceImpl {
	return &NotificationServiceImpl{notifications: notifications}
}

func (s *NotificationServiceImpl) CreateFollow(ctx context.Context, eventID string, followerID, followeeID, occurredAt int64) (bool, error) {
	if !validNotificationEvent(eventID, followerID, followeeID, occurredAt) {
		return false, ErrInvalidNotification
	}
	return s.create(ctx, domain.Notification{
		EventID: eventID, ReceiverID: followeeID, ActorID: followerID,
		Type: domain.NotificationTypeFollow, BizID: followerID, CreatedAt: occurredAt,
	})
}

func (s *NotificationServiceImpl) CreateLike(ctx context.Context, eventID string, actorID, noteID, occurredAt int64) (bool, error) {
	if strings.TrimSpace(eventID) == "" || actorID <= 0 || noteID <= 0 || occurredAt <= 0 {
		return false, ErrInvalidNotification
	}
	receiverID, err := s.notifications.FindNoteAuthorID(ctx, noteID)
	if err != nil {
		return false, err
	}
	return s.create(ctx, domain.Notification{
		EventID: eventID, ReceiverID: receiverID, ActorID: actorID,
		Type: domain.NotificationTypeLike, BizID: noteID, CreatedAt: occurredAt,
	})
}

func (s *NotificationServiceImpl) CreateComment(ctx context.Context, eventID string, actorID, noteID, commentID, occurredAt int64) (bool, error) {
	if strings.TrimSpace(eventID) == "" || actorID <= 0 || noteID <= 0 || commentID <= 0 || occurredAt <= 0 {
		return false, ErrInvalidNotification
	}
	receiverID, err := s.notifications.FindNoteAuthorID(ctx, noteID)
	if err != nil {
		return false, err
	}
	return s.create(ctx, domain.Notification{
		EventID: eventID, ReceiverID: receiverID, ActorID: actorID,
		Type: domain.NotificationTypeComment, BizID: commentID, CreatedAt: occurredAt,
	})
}

func (s *NotificationServiceImpl) List(ctx context.Context, receiverID int64, rawCursor string, limit int) (domain.NotificationPage, error) {
	if receiverID <= 0 {
		return domain.NotificationPage{}, ErrInvalidNotification
	}
	cursor, err := parseNotificationCursor(rawCursor)
	if err != nil {
		return domain.NotificationPage{}, err
	}
	limit = normalizeNotificationLimit(limit)
	rows, err := s.notifications.ListByReceiver(ctx, receiverID, cursor, limit+1)
	if err != nil {
		return domain.NotificationPage{}, err
	}
	page := domain.NotificationPage{Items: rows}
	if len(rows) > limit {
		page.HasMore = true
		page.Items = rows[:limit]
	}
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeNotificationCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}

func (s *NotificationServiceImpl) MarkRead(ctx context.Context, receiverID, notificationID int64) error {
	if receiverID <= 0 || notificationID <= 0 {
		return ErrInvalidNotification
	}
	return s.notifications.MarkRead(ctx, receiverID, notificationID)
}

func (s *NotificationServiceImpl) create(ctx context.Context, notification domain.Notification) (bool, error) {
	if notification.ActorID == notification.ReceiverID {
		return false, nil
	}
	return s.notifications.CreateIfAbsent(ctx, notification)
}

func validNotificationEvent(eventID string, actorID, receiverID, occurredAt int64) bool {
	return strings.TrimSpace(eventID) != "" && actorID > 0 && receiverID > 0 && occurredAt > 0
}

func normalizeNotificationLimit(limit int) int {
	if limit <= 0 {
		return defaultNotificationPageSize
	}
	if limit > maxNotificationPageSize {
		return maxNotificationPageSize
	}
	return limit
}

func encodeNotificationCursor(createdAt, id int64) string {
	return strconv.FormatInt(createdAt, 10) + "_" + strconv.FormatInt(id, 10)
}

func parseNotificationCursor(raw string) (*domain.NotificationCursor, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "_")
	if len(parts) != 2 {
		return nil, ErrInvalidNotificationCursor
	}
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || createdAt <= 0 {
		return nil, ErrInvalidNotificationCursor
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return nil, ErrInvalidNotificationCursor
	}
	return &domain.NotificationCursor{CreatedAt: createdAt, ID: id}, nil
}
