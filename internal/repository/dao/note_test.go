package dao

import (
	"context"
	"errors"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func TestGORMNoteDAO_InsertAndImages(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newNoteMockDB(t)
	defer cleanup()
	mock.ExpectExec("INSERT INTO .*notes.*").WillReturnResult(sqlmock.NewResult(11, 1))
	mock.ExpectExec("INSERT INTO .*note_images.*").WillReturnResult(sqlmock.NewResult(1, 2))

	d := NewGORMNoteDAO(db)
	note, err := d.Insert(context.Background(), NoteOfDB{AuthorId: 3, Title: "t", Content: "c", Status: "published", CreatedAt: 1})
	if err != nil {
		t.Fatalf("insert note: %v", err)
	}
	err = d.InsertImages(context.Background(), []NoteImageOfDB{
		{NoteId: note.Id, URL: "https://a", SortOrder: 0},
		{NoteId: note.Id, URL: "https://b", SortOrder: 1},
	})
	if err != nil {
		t.Fatalf("insert images: %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestGORMNoteDAO_FindByIDNotFound(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newNoteMockDB(t)
	defer cleanup()
	mock.ExpectQuery("SELECT .* FROM .*notes.*").WillReturnError(gorm.ErrRecordNotFound)

	_, err := NewGORMNoteDAO(db).FindByID(context.Background(), 9)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("want ErrNoteNotFound, got %v", err)
	}
}

func TestGORMNoteDAO_SoftDeleteNoRow(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newNoteMockDB(t)
	defer cleanup()
	mock.ExpectExec("UPDATE .*notes.*").WillReturnResult(sqlmock.NewResult(0, 0))

	err := NewGORMNoteDAO(db).SoftDelete(context.Background(), 1, 2)
	if !errors.Is(err, ErrNoteNotFound) {
		t.Fatalf("want ErrNoteNotFound, got %v", err)
	}
}

func newNoteMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, func()) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	db, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), &gorm.Config{
		DisableAutomaticPing:   true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock, func() { _ = sqlDB.Close() }
}
