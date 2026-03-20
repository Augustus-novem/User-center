package dao

import (
	"context"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var ErrUserDuplicateEmail = errors.New("邮件冲突")

type UserDao struct {
	db *gorm.DB
}

func NewUserDao(db *gorm.DB) *UserDao {
	return &UserDao{
		db: db,
	}
}

func (ud *UserDao) Insert(ctx context.Context, user UserOfDB) error {
	user.Ctime = time.Now().UnixMilli()
	user.Utime = time.Now().UnixMilli()
	err := ud.db.WithContext(ctx).Create(&user).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		const uniqueIndexErrNo uint16 = 1062
		if mysqlErr.Number == uniqueIndexErrNo {
			return ErrUserDuplicateEmail
		}
	}
	return err
}

type UserOfDB struct {
	Id       int64  `gorm:"primaryKey; autoIncrement"`
	Email    string `gorm:"type:varchar(255); uniqueIndex"`
	Password string `gorm:"type:varchar(255)"`
	Ctime    int64
	Utime    int64
	Deleted  bool
}
