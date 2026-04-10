package dao

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrPointRecordDuplicate = errors.New("积分流水重复")

type PointDAO interface {
	CreateRecord(ctx context.Context, record UserPointRecordOfDB) error
}

type GORMPointDAO struct {
	db *gorm.DB
}

func NewGORMPointDAO(db *gorm.DB) *GORMPointDAO {
	return &GORMPointDAO{
		db: db,
	}
}

func (d *GORMPointDAO) CreateRecord(ctx context.Context, record UserPointRecordOfDB) error {
	now := time.Now().UnixMilli()
	record.Ctime = now
	record.Utime = now
	err := dbFromCtx(ctx, d.db).Create(&record).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		const uniqueIndexErrNo uint16 = 1062
		if mysqlErr.Number == uniqueIndexErrNo {
			return ErrPointRecordDuplicate
		}
	}
	return err
}

type UserPointRecordOfDB struct {
	Id      int64  `gorm:"primaryKey;autoIncrement"`
	UserId  int64  `gorm:"uniqueIndex:uk_biz_user"`
	BizType string `gorm:"type:varchar(64);uniqueIndex:uk_biz_user"`
	BizId   string `gorm:"type:varchar(128);uniqueIndex:uk_biz_user"`
	Points  int    `gorm:"not null"`
	Ctime   int64
	Utime   int64
}
