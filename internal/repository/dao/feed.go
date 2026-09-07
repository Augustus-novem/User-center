package dao

import (
	"context"

	"gorm.io/gorm"
)

type FeedDAO interface {
	ListFollowingNotes(ctx context.Context, followerID int64, cursor *NoteCursor, limit int) ([]NoteOfDB, error)
}

type GORMFeedDAO struct {
	db *gorm.DB
}

func NewGORMFeedDAO(db *gorm.DB) *GORMFeedDAO {
	return &GORMFeedDAO{db: db}
}

func (d *GORMFeedDAO) ListFollowingNotes(ctx context.Context, followerID int64, cursor *NoteCursor, limit int) ([]NoteOfDB, error) {
	q := dbFromCtx(ctx, d.db).
		Table("notes AS n").
		Select("n.id, n.author_id, n.title, n.content, n.status, n.created_at, n.updated_at").
		Joins("INNER JOIN user_relations AS r ON r.followee_id = n.author_id AND r.follower_id = ?", followerID).
		Where("n.status = ?", "published")
	if cursor != nil {
		q = q.Where("(n.created_at < ?) OR (n.created_at = ? AND n.id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []NoteOfDB
	err := q.Order("n.created_at DESC, n.id DESC").Limit(limit).Scan(&rows).Error
	return rows, err
}
