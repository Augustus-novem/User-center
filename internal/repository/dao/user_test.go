package dao

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMUserDAO_Insert(t *testing.T) {
	errDBDown := errors.New("mock db error")
	testCases := []struct {
		name    string
		prepare func(sqlmock.Sqlmock)
		ctx     context.Context
		user    UserOfDB
		wantErr error
	}{
		{
			name: "插入成功",
			prepare: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO .*user.*").
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			ctx: context.Background(),
			user: UserOfDB{
				Email:    sql.NullString{String: "test@qq.com", Valid: true},
				Password: "hashed-password",
			},
		},
		{
			name: "插入失败-邮箱冲突",
			prepare: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO .*user.*").
					WillReturnError(&mysqlDriver.MySQLError{Number: 1062})
			},
			ctx:     context.Background(),
			user:    UserOfDB{},
			wantErr: ErrUserDuplicate,
		},
		{
			name: "插入失败-普通数据库错误",
			prepare: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec("INSERT INTO .*user.*").
					WillReturnError(errDBDown)
			},
			ctx:     context.Background(),
			user:    UserOfDB{},
			wantErr: errDBDown,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sqlDB, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("create sqlmock failed: %v", err)
			}
			defer sqlDB.Close()

			tc.prepare(mock)

			db, err := gorm.Open(mysql.New(mysql.Config{
				Conn:                      sqlDB,
				SkipInitializeWithVersion: true,
			}), &gorm.Config{
				DisableAutomaticPing:   true,
				SkipDefaultTransaction: true,
			})
			if err != nil {
				t.Fatalf("open gorm with sqlmock failed: %v", err)
			}

			dao := NewGORMUserDAO(db)
			err = dao.Insert(tc.ctx, tc.user)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("want err %v, got %v", tc.wantErr, err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
