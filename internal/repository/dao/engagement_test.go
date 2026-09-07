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

func TestGORMLikeDAO_InsertDuplicate(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newEngagementMockDB(t)
	defer cleanup()
	mock.ExpectExec("INSERT INTO .*note_likes.*").
		WillReturnError(&mysqlDriver.MySQLError{Number: 1062})
	err := NewGORMLikeDAO(db).Insert(context.Background(), NoteLikeOfDB{UserId: 1, NoteId: 2, CreatedAt: 3})
	if !errors.Is(err, ErrLikeDuplicate) {
		t.Fatalf("want ErrLikeDuplicate, got %v", err)
	}
}

func TestGORMCommentDAO_ListByNoteCursor(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newEngagementMockDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"id", "note_id", "user_id", "content", "status", "created_at"}).
		AddRow(2, 9, 1, "hi", "published", 20)
	mock.ExpectQuery("SELECT .* FROM .*comments.*").WillReturnRows(rows)
	got, err := NewGORMCommentDAO(db).ListByNote(context.Background(), 9, "published", &CommentCursor{CreatedAt: 10, ID: 1}, 20)
	if err != nil || len(got) != 1 || got[0].Content != "hi" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func newEngagementMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}
