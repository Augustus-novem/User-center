package dao

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrNotificationNotFound = errors.New("通知不存在")

type NotificationDAO interface {
	InsertIfAbsent(ctx context.Context, notification NotificationOfDB) (bool, error)
	ListByReceiver(ctx context.Context, receiverID int64, cursor *NotificationCursor, limit int) ([]NotificationOfDB, error)
	MarkRead(ctx context.Context, receiverID, notificationID int64) error
}

type NotificationCursor struct {
	CreatedAt int64
	ID        int64
}

type NotificationOfDB struct {
	Id         int64  `gorm:"primaryKey;autoIncrement;index:idx_notification_receiver_created,priority:3"`
	EventId    string `gorm:"column:event_id;type:varchar(64);not null;uniqueIndex:uk_notification_event"`
	ReceiverId int64  `gorm:"column:receiver_id;not null;index:idx_notification_receiver_created,priority:1"`
	ActorId    int64  `gorm:"column:actor_id;not null"`
	Type       string `gorm:"type:varchar(32);not null"`
	BizId      int64  `gorm:"column:biz_id;not null"`
	IsRead     bool   `gorm:"column:is_read;not null;default:false"`
	CreatedAt  int64  `gorm:"column:created_at;not null;index:idx_notification_receiver_created,priority:2"`
}

func (NotificationOfDB) TableName() string { return "notifications" }

type GORMNotificationDAO struct {
	db *gorm.DB
}

func NewGORMNotificationDAO(db *gorm.DB) *GORMNotificationDAO {
	return &GORMNotificationDAO{db: db}
}

func (d *GORMNotificationDAO) InsertIfAbsent(ctx context.Context, notification NotificationOfDB) (bool, error) {
	if notification.CreatedAt == 0 {
		notification.CreatedAt = time.Now().UnixMilli()
	}
	res := dbFromCtx(ctx, d.db).
		Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "event_id"}}, DoNothing: true}).
		Create(&notification)
	return res.RowsAffected == 1, res.Error
}

func (d *GORMNotificationDAO) ListByReceiver(ctx context.Context, receiverID int64, cursor *NotificationCursor, limit int) ([]NotificationOfDB, error) {
	q := dbFromCtx(ctx, d.db).Where("receiver_id = ?", receiverID)
	if cursor != nil {
		q = q.Where("(created_at < ?) OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []NotificationOfDB
	err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (d *GORMNotificationDAO) MarkRead(ctx context.Context, receiverID, notificationID int64) error {
	db := dbFromCtx(ctx, d.db)
	res := db.Model(&NotificationOfDB{}).
		Where("id = ? AND receiver_id = ?", notificationID, receiverID).
		Update("is_read", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected > 0 {
		return nil
	}
	var count int64
	if err := db.Model(&NotificationOfDB{}).
		Where("id = ? AND receiver_id = ?", notificationID, receiverID).
		Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return ErrNotificationNotFound
	}
	return nil
}
