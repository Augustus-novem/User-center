package dao

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

var (
	ErrDataNotFound  = gorm.ErrRecordNotFound
	ErrUserDuplicate = errors.New("用户已存在")
)

type UserDAO struct {
	db *gorm.DB
}

func NewUserDao(db *gorm.DB) *UserDAO {
	return &UserDAO{
		db: db,
	}
}

func (ud *UserDAO) Insert(ctx context.Context, user UserOfDB) error {
	user.Ctime = time.Now().UnixMilli()
	user.Utime = time.Now().UnixMilli()
	err := ud.db.WithContext(ctx).Create(&user).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		const uniqueIndexErrNo uint16 = 1062
		if mysqlErr.Number == uniqueIndexErrNo {
			return ErrUserDuplicate
		}
	}
	return err
}

func (ud *UserDAO) FindByEmail(ctx context.Context, email string) (UserOfDB, error) {
	var user UserOfDB
	err := ud.db.WithContext(ctx).
		Where("email = ?", email).
		Take(&user).Error
	return user, err
}

func (ud *UserDAO) FindById(ctx context.Context, Id int64) (UserOfDB, error) {
	var user UserOfDB
	err := ud.db.WithContext(ctx).
		Where("id = ?", Id).
		First(&user).Error
	return user, err
}

func (ud *UserDAO) FindByPhone(ctx context.Context, phone string) (UserOfDB, error) {
	var user UserOfDB
	err := ud.db.WithContext(ctx).
		Where("phone= ?", phone).
		First(&user).Error
	return user, err
}

type UserOfDB struct {
	Id       int64          `gorm:"primaryKey; autoIncrement"`
	Email    sql.NullString `gorm:"type:varchar(255); uniqueIndex"`
	Password string         `gorm:"type:varchar(255)"`
	Phone    sql.NullString `gorm:"type:varchar(255);uniqueIndex"`
	Ctime    int64
	Utime    int64
	Deleted  bool
}
