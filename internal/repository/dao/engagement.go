package dao

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrLikeDuplicate = errors.New("已经点赞")

type LikeDAO interface {
	Insert(ctx context.Context, like NoteLikeOfDB) error
	Delete(ctx context.Context, userID, noteID int64) error
}

type CommentDAO interface {
	Insert(ctx context.Context, comment CommentOfDB) (CommentOfDB, error)
	ListByNote(ctx context.Context, noteID int64, status string, cursor *CommentCursor, limit int) ([]CommentOfDB, error)
}

type CommentCursor struct {
	CreatedAt int64
	ID        int64
}

type NoteLikeOfDB struct {
	Id        int64 `gorm:"primaryKey;autoIncrement"`
	UserId    int64 `gorm:"column:user_id;not null;uniqueIndex:uk_note_like_user_note"`
	NoteId    int64 `gorm:"column:note_id;not null;uniqueIndex:uk_note_like_user_note"`
	CreatedAt int64 `gorm:"column:created_at;not null"`
}

func (NoteLikeOfDB) TableName() string { return "note_likes" }

type CommentOfDB struct {
	Id        int64  `gorm:"primaryKey;autoIncrement;index:idx_note_comment_created,priority:4"`
	NoteId    int64  `gorm:"column:note_id;not null;index:idx_note_comment_created,priority:1"`
	UserId    int64  `gorm:"column:user_id;not null"`
	Content   string `gorm:"type:varchar(500);not null"`
	Status    string `gorm:"type:varchar(16);not null;index:idx_note_comment_created,priority:2"`
	CreatedAt int64  `gorm:"column:created_at;not null;index:idx_note_comment_created,priority:3"`
}

func (CommentOfDB) TableName() string { return "comments" }

type GORMLikeDAO struct{ db *gorm.DB }

func NewGORMLikeDAO(db *gorm.DB) *GORMLikeDAO { return &GORMLikeDAO{db: db} }

func (d *GORMLikeDAO) Insert(ctx context.Context, like NoteLikeOfDB) error {
	if like.CreatedAt == 0 {
		like.CreatedAt = time.Now().UnixMilli()
	}
	err := dbFromCtx(ctx, d.db).Create(&like).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == 1062 {
		return ErrLikeDuplicate
	}
	return err
}

func (d *GORMLikeDAO) Delete(ctx context.Context, userID, noteID int64) error {
	return dbFromCtx(ctx, d.db).
		Where("user_id = ? AND note_id = ?", userID, noteID).
		Delete(&NoteLikeOfDB{}).Error
}

type GORMCommentDAO struct{ db *gorm.DB }

func NewGORMCommentDAO(db *gorm.DB) *GORMCommentDAO { return &GORMCommentDAO{db: db} }

func (d *GORMCommentDAO) Insert(ctx context.Context, comment CommentOfDB) (CommentOfDB, error) {
	if comment.CreatedAt == 0 {
		comment.CreatedAt = time.Now().UnixMilli()
	}
	err := dbFromCtx(ctx, d.db).Create(&comment).Error
	return comment, err
}

func (d *GORMCommentDAO) ListByNote(ctx context.Context, noteID int64, status string, cursor *CommentCursor, limit int) ([]CommentOfDB, error) {
	q := dbFromCtx(ctx, d.db).Where("note_id = ? AND status = ?", noteID, status)
	if cursor != nil {
		q = q.Where("(created_at > ?) OR (created_at = ? AND id > ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []CommentOfDB
	err := q.Order("created_at ASC, id ASC").Limit(limit).Find(&rows).Error
	return rows, err
}
