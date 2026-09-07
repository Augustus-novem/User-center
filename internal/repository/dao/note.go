package dao

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

var ErrNoteNotFound = errors.New("笔记不存在")

type NoteDAO interface {
	Insert(ctx context.Context, note NoteOfDB) (NoteOfDB, error)
	InsertImages(ctx context.Context, images []NoteImageOfDB) error
	FindByID(ctx context.Context, id int64) (NoteOfDB, error)
	FindByIDs(ctx context.Context, ids []int64) ([]NoteOfDB, error)
	ListImages(ctx context.Context, noteID int64) ([]NoteImageOfDB, error)
	ListByAuthor(ctx context.Context, authorID int64, status string, cursor *NoteCursor, limit int) ([]NoteOfDB, error)
	ListPublishedBefore(ctx context.Context, authorID, exclusiveMaxID int64, limit int) ([]NoteOfDB, error)
	SoftDelete(ctx context.Context, id, authorID int64) error
}

type NoteCursor struct {
	CreatedAt int64
	ID        int64
}

type NoteOfDB struct {
	Id        int64  `gorm:"primaryKey;autoIncrement;index:idx_author_status_created,priority:4"`
	AuthorId  int64  `gorm:"column:author_id;not null;index:idx_author_status_created,priority:1"`
	Title     string `gorm:"type:varchar(80);not null"`
	Content   string `gorm:"type:text"`
	Status    string `gorm:"type:varchar(16);not null;index:idx_author_status_created,priority:2"`
	CreatedAt int64  `gorm:"column:created_at;not null;index:idx_author_status_created,priority:3"`
	UpdatedAt int64  `gorm:"column:updated_at;not null"`
}

func (NoteOfDB) TableName() string {
	return "notes"
}

type NoteImageOfDB struct {
	Id        int64  `gorm:"primaryKey;autoIncrement"`
	NoteId    int64  `gorm:"column:note_id;not null;uniqueIndex:uk_note_image_sort,priority:1;index:idx_note_image_sort,priority:1"`
	URL       string `gorm:"column:url;type:varchar(1024);not null"`
	SortOrder int    `gorm:"column:sort_order;not null;uniqueIndex:uk_note_image_sort,priority:2;index:idx_note_image_sort,priority:2"`
}

func (NoteImageOfDB) TableName() string {
	return "note_images"
}

type GORMNoteDAO struct {
	db *gorm.DB
}

func NewGORMNoteDAO(db *gorm.DB) *GORMNoteDAO {
	return &GORMNoteDAO{db: db}
}

func (d *GORMNoteDAO) Insert(ctx context.Context, note NoteOfDB) (NoteOfDB, error) {
	now := time.Now().UnixMilli()
	if note.CreatedAt == 0 {
		note.CreatedAt = now
	}
	note.UpdatedAt = now
	err := dbFromCtx(ctx, d.db).Create(&note).Error
	return note, err
}

func (d *GORMNoteDAO) InsertImages(ctx context.Context, images []NoteImageOfDB) error {
	if len(images) == 0 {
		return nil
	}
	return dbFromCtx(ctx, d.db).Create(&images).Error
}

func (d *GORMNoteDAO) FindByID(ctx context.Context, id int64) (NoteOfDB, error) {
	var note NoteOfDB
	err := dbFromCtx(ctx, d.db).Where("id = ?", id).Take(&note).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return NoteOfDB{}, ErrNoteNotFound
	}
	return note, err
}

func (d *GORMNoteDAO) ListImages(ctx context.Context, noteID int64) ([]NoteImageOfDB, error) {
	var images []NoteImageOfDB
	err := dbFromCtx(ctx, d.db).
		Where("note_id = ?", noteID).
		Order("sort_order ASC, id ASC").
		Find(&images).Error
	return images, err
}

func (d *GORMNoteDAO) ListByAuthor(ctx context.Context, authorID int64, status string, cursor *NoteCursor, limit int) ([]NoteOfDB, error) {
	q := dbFromCtx(ctx, d.db).Where("author_id = ? AND status = ?", authorID, status)
	if cursor != nil {
		q = q.Where("(created_at < ?) OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []NoteOfDB
	err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (d *GORMNoteDAO) ListPublishedBefore(ctx context.Context, authorID, exclusiveMaxID int64, limit int) ([]NoteOfDB, error) {
	if limit <= 0 {
		return nil, nil
	}
	q := dbFromCtx(ctx, d.db).Where("author_id = ? AND status = ?", authorID, "published")
	if exclusiveMaxID > 0 {
		q = q.Where("id < ?", exclusiveMaxID)
	}
	var rows []NoteOfDB
	err := q.Order("id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}

func (d *GORMNoteDAO) FindByIDs(ctx context.Context, ids []int64) ([]NoteOfDB, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []NoteOfDB
	err := dbFromCtx(ctx, d.db).Where("id IN ?", ids).Find(&rows).Error
	return rows, err
}

func (d *GORMNoteDAO) SoftDelete(ctx context.Context, id, authorID int64) error {
	now := time.Now().UnixMilli()
	tx := dbFromCtx(ctx, d.db).
		Model(&NoteOfDB{}).
		Where("id = ? AND author_id = ? AND status = ?", id, authorID, "published").
		Updates(map[string]any{
			"status":     "deleted",
			"updated_at": now,
		})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return ErrNoteNotFound
	}
	return nil
}
