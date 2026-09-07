package dao

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMFollowDAO_Insert(t *testing.T) {
	t.Parallel()
	errDBDown := errors.New("mock db error")

	t.Run("success", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newFollowMockDB(t)
		defer cleanup()
		mock.ExpectExec("INSERT INTO .*user_relations.*").
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := NewGORMFollowDAO(db).Insert(context.Background(), UserRelationOfDB{
			FollowerId: 1,
			FolloweeId: 2,
			CreatedAt:  100,
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := mock.ExpectationsWereMet(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("duplicate maps to ErrFollowDuplicate", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newFollowMockDB(t)
		defer cleanup()
		mock.ExpectExec("INSERT INTO .*user_relations.*").
			WillReturnError(&mysqlDriver.MySQLError{Number: 1062})

		err := NewGORMFollowDAO(db).Insert(context.Background(), UserRelationOfDB{
			FollowerId: 1,
			FolloweeId: 2,
			CreatedAt:  100,
		})
		if !errors.Is(err, ErrFollowDuplicate) {
			t.Fatalf("want ErrFollowDuplicate, got %v", err)
		}
	})

	t.Run("other db error", func(t *testing.T) {
		t.Parallel()
		db, mock, cleanup := newFollowMockDB(t)
		defer cleanup()
		mock.ExpectExec("INSERT INTO .*user_relations.*").
			WillReturnError(errDBDown)

		err := NewGORMFollowDAO(db).Insert(context.Background(), UserRelationOfDB{
			FollowerId: 1,
			FolloweeId: 2,
			CreatedAt:  100,
		})
		if !errors.Is(err, errDBDown) {
			t.Fatalf("want %v, got %v", errDBDown, err)
		}
	})
}

func TestGORMFollowDAO_Delete(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newFollowMockDB(t)
	defer cleanup()
	mock.ExpectExec("DELETE FROM .*user_relations.*").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewGORMFollowDAO(db).Delete(context.Background(), 1, 2)
	if err != nil {
		t.Fatalf("duplicate unfollow should succeed, got %v", err)
	}
}

func TestGORMFollowDAO_ListFollowing_usesStableCursor(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newFollowMockDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"id", "follower_id", "followee_id", "created_at"}).
		AddRow(8, 1, 3, 200)
	mock.ExpectQuery("SELECT .* FROM .*user_relations.*").
		WillReturnRows(rows)

	got, err := NewGORMFollowDAO(db).ListFollowing(context.Background(), 1, &FollowCursor{CreatedAt: 300, ID: 9}, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].FolloweeId != 3 {
		t.Fatalf("unexpected rows: %+v", got)
	}
}

func TestGORMFollowDAO_ListFolloweeIDs(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newFollowMockDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"followee_id"}).AddRow(8).AddRow(9)
	mock.ExpectQuery("SELECT .*followee_id.* FROM .*user_relations.*").WillReturnRows(rows)

	got, err := NewGORMFollowDAO(db).ListFolloweeIDs(context.Background(), 1, []int64{8, 9, 10})
	if err != nil {
		t.Fatalf("ListFolloweeIDs: %v", err)
	}
	if len(got) != 2 || got[0] != 8 || got[1] != 9 {
		t.Fatalf("unexpected ids: %v", got)
	}
}

func newFollowMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("create sqlmock failed: %v", err)
	}
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
	return db, mock, func() { _ = sqlDB.Close() }
}
