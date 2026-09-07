package dao

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrFollowDuplicate = errors.New("关注关系已存在")

type FollowDAO interface {
	Insert(ctx context.Context, rel UserRelationOfDB) error
	Delete(ctx context.Context, followerID, followeeID int64) error
	ListFollowing(ctx context.Context, followerID int64, cursor *FollowCursor, limit int) ([]UserRelationOfDB, error)
	ListFollowers(ctx context.Context, followeeID int64, cursor *FollowCursor, limit int) ([]UserRelationOfDB, error)
}

type FollowCursor struct {
	CreatedAt int64
	ID        int64
}

type UserRelationOfDB struct {
	Id         int64 `gorm:"primaryKey;autoIncrement;index:idx_follower_created,priority:3;index:idx_followee_created,priority:3"`
	FollowerId int64 `gorm:"column:follower_id;not null;uniqueIndex:uk_user_relation_follower_followee;index:idx_follower_created,priority:1"`
	FolloweeId int64 `gorm:"column:followee_id;not null;uniqueIndex:uk_user_relation_follower_followee;index:idx_followee_created,priority:1"`
	CreatedAt  int64 `gorm:"column:created_at;not null;index:idx_follower_created,priority:2;index:idx_followee_created,priority:2"`
}

func (UserRelationOfDB) TableName() string {
	return "user_relations"
}

type GORMFollowDAO struct {
	db *gorm.DB
}

func NewGORMFollowDAO(db *gorm.DB) *GORMFollowDAO {
	return &GORMFollowDAO{db: db}
}

func (d *GORMFollowDAO) Insert(ctx context.Context, rel UserRelationOfDB) error {
	if rel.CreatedAt == 0 {
		rel.CreatedAt = time.Now().UnixMilli()
	}
	err := dbFromCtx(ctx, d.db).Create(&rel).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		const uniqueIndexErrNo uint16 = 1062
		if mysqlErr.Number == uniqueIndexErrNo {
			return ErrFollowDuplicate
		}
	}
	return err
}

func (d *GORMFollowDAO) Delete(ctx context.Context, followerID, followeeID int64) error {
	return dbFromCtx(ctx, d.db).
		Where("follower_id = ? AND followee_id = ?", followerID, followeeID).
		Delete(&UserRelationOfDB{}).Error
}

func (d *GORMFollowDAO) ListFollowing(ctx context.Context, followerID int64, cursor *FollowCursor, limit int) ([]UserRelationOfDB, error) {
	return d.listBy(ctx, "follower_id", followerID, cursor, limit)
}

func (d *GORMFollowDAO) ListFollowers(ctx context.Context, followeeID int64, cursor *FollowCursor, limit int) ([]UserRelationOfDB, error) {
	return d.listBy(ctx, "followee_id", followeeID, cursor, limit)
}

func (d *GORMFollowDAO) listBy(ctx context.Context, column string, userID int64, cursor *FollowCursor, limit int) ([]UserRelationOfDB, error) {
	q := dbFromCtx(ctx, d.db).Where(column+" = ?", userID)
	if cursor != nil {
		q = q.Where("(created_at < ?) OR (created_at = ? AND id < ?)", cursor.CreatedAt, cursor.CreatedAt, cursor.ID)
	}
	var rows []UserRelationOfDB
	err := q.Order("created_at DESC, id DESC").Limit(limit).Find(&rows).Error
	return rows, err
}
