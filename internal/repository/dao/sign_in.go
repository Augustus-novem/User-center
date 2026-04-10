package dao

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrSignInDuplicate = errors.New("签到重复")
)

type SignInDAO interface {
	CreateRecord(ctx context.Context, record UserSignInRecordOfDB) error
	GetStat(ctx context.Context, userID int64) (UserSignInStatOfDB, error)
	CreateStat(ctx context.Context, stat UserSignInStatOfDB) error
	UpdateStat(ctx context.Context, stat UserSignInStatOfDB) error
	HasSignedOnBizDay(ctx context.Context, userID int64, bizDay int) (bool, error)
	ListSignedBizDaysInMonth(ctx context.Context, userId int64, monthStartMs, nextMonthStartMs int64) ([]int, error)
}

type GORMSignInDAO struct {
	db *gorm.DB
}

func NewGORMSignInDAO(db *gorm.DB) *GORMSignInDAO {
	return &GORMSignInDAO{db: db}
}

func (d *GORMSignInDAO) CreateRecord(ctx context.Context, record UserSignInRecordOfDB) error {
	now := time.Now().UnixMilli()
	record.Ctime = now
	record.Utime = now
	err := dbFromCtx(ctx, d.db).Create(&record).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		const uniqueIndexErrNo uint16 = 1062
		if mysqlErr.Number == uniqueIndexErrNo {
			return ErrSignInDuplicate
		}
	}
	return err
}

func (d *GORMSignInDAO) GetStat(ctx context.Context, userID int64) (UserSignInStatOfDB, error) {
	var stat UserSignInStatOfDB
	err := dbFromCtx(ctx, d.db).
		Where("user_id = ?", userID).
		Take(&stat).Error
	return stat, err
}

func (d *GORMSignInDAO) CreateStat(ctx context.Context, stat UserSignInStatOfDB) error {
	now := time.Now().UnixMilli()
	stat.Ctime = now
	stat.Utime = now
	return dbFromCtx(ctx, d.db).Create(&stat).Error
}

func (d *GORMSignInDAO) UpdateStat(ctx context.Context, stat UserSignInStatOfDB) error {
	stat.Utime = time.Now().UnixMilli()
	return dbFromCtx(ctx, d.db).Model(&UserSignInStatOfDB{}).
		Where("id = ?", stat.Id).
		Updates(map[string]any{
			"continuous_days": stat.ContinuousDays,
			"total_days":      stat.TotalDays,
			"last_sign_at":    stat.LastSignAt,
			"utime":           stat.Utime,
		}).Error
}

func (d *GORMSignInDAO) HasSignedOnBizDay(ctx context.Context, userID int64, bizDay int) (bool, error) {
	var cnt int64
	err := dbFromCtx(ctx, d.db).Model(&UserSignInRecordOfDB{}).
		Where("user_id = ? AND biz_day = ?", userID, bizDay).
		Count(&cnt).Error
	return cnt > 0, err
}

func (d *GORMSignInDAO) ListSignedBizDaysInMonth(ctx context.Context, userId int64, monthStartMs, nextMonthStartMs int64) ([]int, error) {
	var records []UserSignInRecordOfDB
	err := dbFromCtx(ctx, d.db).
		Where("user_id = ? AND sign_in_at >= ? AND sign_in_at < ?", userId, monthStartMs, nextMonthStartMs).
		Order("sign_in_at ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	res := make([]int, 0, len(records))
	for _, record := range records {
		res = append(res, record.BizDay)
	}
	return res, nil
}

type UserSignInRecordOfDB struct {
	Id       int64 `gorm:"primaryKey;autoIncrement"`
	UserId   int64 `gorm:"not null;index:idx_user_biz_day;uniqueIndex:uk_user_biz_day"`
	BizDay   int   `gorm:"not null;index:idx_user_biz_day;uniqueIndex:uk_user_biz_day"`
	SignInAt int64 `gorm:"not null;index"`
	Ctime    int64
	Utime    int64
}

type UserSignInStatOfDB struct {
	Id             int64 `gorm:"primaryKey;autoIncrement"`
	UserId         int64 `gorm:"uniqueIndex"`
	ContinuousDays int   `gorm:"not null;default:0"`
	TotalDays      int   `gorm:"not null;default:0"`
	LastSignAt     int64 `gorm:"not null;default:0"`
	Ctime          int64
	Utime          int64
}
