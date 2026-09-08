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

func TestGORMNoteDAO_FindByIDs(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newNoteMockDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"id", "author_id", "title", "content", "status", "created_at", "updated_at"}).
		AddRow(2, 8, "t", "c", "published", 1, 1).
		AddRow(3, 8, "d", "c", "deleted", 1, 1)
	mock.ExpectQuery("SELECT .* FROM .*notes.*").WillReturnRows(rows)

	got, err := NewGORMNoteDAO(db).FindByIDs(context.Background(), []int64{2, 3, 4})
	if err != nil {
		t.Fatalf("FindByIDs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 rows including deleted, got %+v", got)
	}
}

func TestGORMNoteDAO_ListPublishedAfterIDIsBounded(t *testing.T) {
	t.Parallel()
	db, mock, cleanup := newNoteMockDB(t)
	defer cleanup()
	rows := sqlmock.NewRows([]string{"id", "author_id", "title", "content", "status", "created_at", "updated_at"}).
		AddRow(11, 8, "t", "c", "published", 1, 1)
	mock.ExpectQuery("SELECT .* FROM .*notes.* WHERE status = \\? AND id > \\? ORDER BY id ASC LIMIT \\?").
		WithArgs("published", int64(10), 2).
		WillReturnRows(rows)

	got, err := NewGORMNoteDAO(db).ListPublishedAfterID(context.Background(), 10, 2)
	if err != nil {
		t.Fatalf("ListPublishedAfterID: %v", err)
	}
	if len(got) != 1 || got[0].Id != 11 {
		t.Fatalf("unexpected rows: %+v", got)
	}
	if err = mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
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
