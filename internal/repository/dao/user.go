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

type UserDAO interface {
	Insert(ctx context.Context, user UserOfDB) error
	FindByEmail(ctx context.Context, email string) (UserOfDB, error)
	FindById(ctx context.Context, id int64) (UserOfDB, error)
	FindByPhone(ctx context.Context, phone string) (UserOfDB, error)
	InsertAndReturn(ctx context.Context, u UserOfDB) (UserOfDB, error)
	UpdateNonSensitive(ctx context.Context, u UserOfDB) error
}

type GORMUserDAO struct {
	db *gorm.DB
}

func NewGORMUserDAO(db *gorm.DB) *GORMUserDAO {
	return &GORMUserDAO{
		db: db,
	}
}

func (ud *GORMUserDAO) Insert(ctx context.Context, user UserOfDB) error {
	_, err := ud.InsertAndReturn(ctx, user)
	return err
}

func (ud *GORMUserDAO) InsertAndReturn(ctx context.Context, user UserOfDB) (UserOfDB, error) {
	now := time.Now().UnixMilli()
	user.Ctime = now
	user.Utime = now
	err := dbFromCtx(ctx, ud.db).Create(&user).Error
	var mysqlErr *mysql.MySQLError
	if errors.As(err, &mysqlErr) {
		const uniqueIndexErrNo uint16 = 1062
		if mysqlErr.Number == uniqueIndexErrNo {
			return user, ErrUserDuplicate
		}
	}
	return user, err
}

func (ud *GORMUserDAO) UpdateNonSensitive(ctx context.Context, user UserOfDB) error {
	user.Utime = time.Now().UnixMilli()
	return dbFromCtx(ctx, ud.db).Model(&UserOfDB{}).
		Where("id = ?", user.Id).
		Updates(map[string]any{
			"nickname": user.Nickname,
			"about_me": user.AboutMe,
			"birthday": user.Birthday,
			"utime":    user.Utime,
		}).Error
}

func (ud *GORMUserDAO) FindByEmail(ctx context.Context, email string) (UserOfDB, error) {
	var user UserOfDB
	err := dbFromCtx(ctx, ud.db).
		Where("email = ?", email).
		Take(&user).Error
	return user, err
}

func (ud *GORMUserDAO) FindById(ctx context.Context, Id int64) (UserOfDB, error) {
	var user UserOfDB
	err := dbFromCtx(ctx, ud.db).
		Where("id = ?", Id).
		First(&user).Error
	return user, err
}

func (ud *GORMUserDAO) FindByPhone(ctx context.Context, phone string) (UserOfDB, error) {
	var user UserOfDB
	err := dbFromCtx(ctx, ud.db).
		Where("phone= ?", phone).
		First(&user).Error
	return user, err
}

type UserOfDB struct {
	Id       int64          `gorm:"primaryKey; autoIncrement"`
	Email    sql.NullString `gorm:"type:varchar(255); uniqueIndex"`
	Password string         `gorm:"type:varchar(255)"`
	Phone    sql.NullString `gorm:"type:varchar(255);uniqueIndex"`
	Birthday sql.NullInt64
	// 昵称
	Nickname sql.NullString
	AboutMe  sql.NullString `gorm:"type=varchar(1024)"`
	Ctime    int64
	Utime    int64
	Deleted  bool
}
