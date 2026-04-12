package dao

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

var (
	ErrEventOutboxNotFound = errors.New("event outbox not found")
)

const (
	EventOutboxStatusPending   = "pending"
	EventOutboxStatusPublished = "published"
)

type EventOutboxDAO interface {
	Insert(ctx context.Context, evt EventOutboxOfDB) (EventOutboxOfDB, error)
	ListPending(ctx context.Context, limit int) ([]EventOutboxOfDB, error)
	MarkPublished(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, reason string) error
}

type GORMEventOutboxDAO struct {
	db *gorm.DB
}

func NewGORMEventOutboxDAO(db *gorm.DB) *GORMEventOutboxDAO {
	return &GORMEventOutboxDAO{db: db}
}

func (d *GORMEventOutboxDAO) Insert(ctx context.Context, evt EventOutboxOfDB) (EventOutboxOfDB, error) {
	now := time.Now().UnixMilli()
	evt.Status = EventOutboxStatusPending
	evt.Ctime = now
	evt.Utime = now
	err := dbFromCtx(ctx, d.db).Create(&evt).Error
	return evt, err
}

func (d *GORMEventOutboxDAO) ListPending(ctx context.Context, limit int) ([]EventOutboxOfDB, error) {
	if limit <= 0 {
		limit = 100
	}
	var res []EventOutboxOfDB
	err := dbFromCtx(ctx, d.db).
		Where("status = ?", EventOutboxStatusPending).
		Order("id ASC").
		Limit(limit).
		Find(&res).Error
	return res, err
}

func (d *GORMEventOutboxDAO) MarkPublished(ctx context.Context, id int64) error {
	now := time.Now().UnixMilli()
	tx := dbFromCtx(ctx, d.db).
		Model(&EventOutboxOfDB{}).
		Where("id = ? AND status = ?", id, EventOutboxStatusPending).
		Updates(map[string]any{
			"status":       EventOutboxStatusPublished,
			"published_at": now,
			"utime":        now,
			"last_error":   "",
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrEventOutboxNotFound
	}
	return nil
}

func (d *GORMEventOutboxDAO) MarkFailed(ctx context.Context, id int64, reason string) error {
	now := time.Now().UnixMilli()
	tx := dbFromCtx(ctx, d.db).
		Model(&EventOutboxOfDB{}).
		Where("id = ? AND status = ?", id, EventOutboxStatusPending).
		Updates(map[string]any{
			"attempts":   gorm.Expr("attempts + 1"),
			"last_error": truncateOutboxErr(reason),
			"utime":      now,
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrEventOutboxNotFound
	}
	return nil
}

func truncateOutboxErr(reason string) string {
	reason = strings.TrimSpace(reason)
	if len(reason) <= 1000 {
		return reason
	}
	return reason[:1000]
}

type EventOutboxOfDB struct {
	ID          int64  `gorm:"primaryKey;autoIncrement"`
	Topic       string `gorm:"type:varchar(128);index:idx_outbox_status_id,priority:2;not null"`
	MessageKey  string `gorm:"column:message_key;type:varchar(255);not null"`
	Payload     []byte `gorm:"type:blob;not null"`
	Status      string `gorm:"type:varchar(32);index:idx_outbox_status_id,priority:1;not null"`
	Attempts    int    `gorm:"not null;default:0"`
	LastError   string `gorm:"type:text"`
	PublishedAt int64
	Ctime       int64 `gorm:"index"`
	Utime       int64
}

func (EventOutboxOfDB) TableName() string {
	return "event_outbox"
}
