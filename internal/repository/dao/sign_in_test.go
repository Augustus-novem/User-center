package dao

import (
	"context"
	"errors"
	"regexp"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func newMockGormDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock failed: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("open gorm with sqlmock failed: %v", err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}

func TestGORMSignInDAO_CreateRecord(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newMockGormDB(t)
		defer cleanup()
		mock.ExpectExec("INSERT INTO .*user_sign_in_record_of_dbs.*").WillReturnResult(sqlmock.NewResult(1, 1))

		dao := NewGORMSignInDAO(db)
		err := dao.CreateRecord(context.Background(), UserSignInRecordOfDB{UserId: 1, BizDay: 20260410, SignInAt: 123})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("duplicate maps to domain error", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newMockGormDB(t)
		defer cleanup()
		mock.ExpectExec("INSERT INTO .*user_sign_in_record_of_dbs.*").WillReturnError(&mysqlDriver.MySQLError{Number: 1062})

		dao := NewGORMSignInDAO(db)
		err := dao.CreateRecord(context.Background(), UserSignInRecordOfDB{})
		if !errors.Is(err, ErrSignInDuplicate) {
			t.Fatalf("want err %v, got %v", ErrSignInDuplicate, err)
		}
	})
}

func TestGORMSignInDAO_GetStat(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"id", "user_id", "continuous_days", "total_days", "last_sign_at", "ctime", "utime"}).
		AddRow(2, 7, 3, 10, 123456, 1, 2)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_sign_in_stat_of_dbs` WHERE user_id = ? LIMIT ?")).
		WithArgs(int64(7), 1).WillReturnRows(rows)

	dao := NewGORMSignInDAO(db)
	stat, err := dao.GetStat(context.Background(), 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stat.Id != 2 || stat.UserId != 7 || stat.ContinuousDays != 3 || stat.TotalDays != 10 || stat.LastSignAt != 123456 {
		t.Fatalf("unexpected stat: %+v", stat)
	}
}

func TestGORMSignInDAO_CreateAndUpdateStat(t *testing.T) {
	t.Parallel()

	t.Run("create stat", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newMockGormDB(t)
		defer cleanup()
		mock.ExpectExec("INSERT INTO .*user_sign_in_stat_of_dbs.*").WillReturnResult(sqlmock.NewResult(1, 1))
		dao := NewGORMSignInDAO(db)
		err := dao.CreateStat(context.Background(), UserSignInStatOfDB{UserId: 7, ContinuousDays: 1, TotalDays: 1, LastSignAt: 100})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("update stat", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newMockGormDB(t)
		defer cleanup()
		mock.ExpectExec("UPDATE .*user_sign_in_stat_of_dbs.*").WillReturnResult(sqlmock.NewResult(0, 1))
		dao := NewGORMSignInDAO(db)
		err := dao.UpdateStat(context.Background(), UserSignInStatOfDB{Id: 9, ContinuousDays: 5, TotalDays: 20, LastSignAt: 333})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestGORMSignInDAO_HasSignedOnBizDay(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"count(*)"}).AddRow(1)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT count(*) FROM `user_sign_in_record_of_dbs` WHERE user_id = ? AND biz_day = ?")).
		WithArgs(int64(8), 20260410).WillReturnRows(rows)

	dao := NewGORMSignInDAO(db)
	ok, err := dao.HasSignedOnBizDay(context.Background(), 8, 20260410)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatal("want signed=true")
	}
}

func TestGORMSignInDAO_ListSignedBizDaysInMonth(t *testing.T) {
	t.Parallel()

	db, mock, cleanup := newMockGormDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"id", "user_id", "biz_day", "sign_in_at", "ctime", "utime"}).
		AddRow(1, 9, 20260401, 1000, 1, 1).
		AddRow(2, 9, 20260403, 3000, 1, 1)
	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `user_sign_in_record_of_dbs` WHERE user_id = ? AND sign_in_at >= ? AND sign_in_at < ? ORDER BY sign_in_at ASC")).
		WithArgs(int64(9), int64(1000), int64(5000)).WillReturnRows(rows)

	dao := NewGORMSignInDAO(db)
	days, err := dao.ListSignedBizDaysInMonth(context.Background(), 9, 1000, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(days) != 2 || days[0] != 20260401 || days[1] != 20260403 {
		t.Fatalf("unexpected days: %v", days)
	}
}
