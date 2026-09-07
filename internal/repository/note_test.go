package repository

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

type noteDAOStub struct {
	insertFn       func(ctx context.Context, note dao.NoteOfDB) (dao.NoteOfDB, error)
	insertImagesFn func(ctx context.Context, images []dao.NoteImageOfDB) error
	findByIDFn     func(ctx context.Context, id int64) (dao.NoteOfDB, error)
	listImagesFn   func(ctx context.Context, noteID int64) ([]dao.NoteImageOfDB, error)
	listByAuthorFn func(ctx context.Context, authorID int64, status string, cursor *dao.NoteCursor, limit int) ([]dao.NoteOfDB, error)
	softDeleteFn   func(ctx context.Context, id, authorID int64) error
}

func (s *noteDAOStub) Insert(ctx context.Context, note dao.NoteOfDB) (dao.NoteOfDB, error) {
	if s.insertFn == nil {
		note.Id = 1
		return note, nil
	}
	return s.insertFn(ctx, note)
}

func (s *noteDAOStub) InsertImages(ctx context.Context, images []dao.NoteImageOfDB) error {
	if s.insertImagesFn == nil {
		return nil
	}
	return s.insertImagesFn(ctx, images)
}

func (s *noteDAOStub) FindByID(ctx context.Context, id int64) (dao.NoteOfDB, error) {
	if s.findByIDFn == nil {
		return dao.NoteOfDB{}, dao.ErrNoteNotFound
	}
	return s.findByIDFn(ctx, id)
}

func (s *noteDAOStub) ListImages(ctx context.Context, noteID int64) ([]dao.NoteImageOfDB, error) {
	if s.listImagesFn == nil {
		return nil, nil
	}
	return s.listImagesFn(ctx, noteID)
}

func (s *noteDAOStub) ListByAuthor(ctx context.Context, authorID int64, status string, cursor *dao.NoteCursor, limit int) ([]dao.NoteOfDB, error) {
	if s.listByAuthorFn == nil {
		return nil, nil
	}
	return s.listByAuthorFn(ctx, authorID, status, cursor, limit)
}

func (s *noteDAOStub) SoftDelete(ctx context.Context, id, authorID int64) error {
	if s.softDeleteFn == nil {
		return nil
	}
	return s.softDeleteFn(ctx, id, authorID)
}

func TestNoteRepositoryImpl_CreateKeepsImageOrder(t *testing.T) {
	t.Parallel()
	var got []dao.NoteImageOfDB
	repo := NewNoteRepositoryImpl(&noteDAOStub{
		insertFn: func(ctx context.Context, note dao.NoteOfDB) (dao.NoteOfDB, error) {
			note.Id = 5
			return note, nil
		},
		insertImagesFn: func(ctx context.Context, images []dao.NoteImageOfDB) error {
			got = images
			return nil
		},
	})
	note, err := repo.Create(context.Background(), domain.Note{
		AuthorID: 8,
		Title:    "t",
		Images: []domain.NoteImage{
			{URL: "https://b", SortOrder: 0},
			{URL: "https://a", SortOrder: 1},
		},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if note.ID != 5 || len(got) != 2 || got[0].URL != "https://b" || got[0].SortOrder != 0 || got[1].SortOrder != 1 {
		t.Fatalf("unexpected images: note=%+v images=%+v", note, got)
	}
}

func TestNoteRepositoryImpl_FindByIDDeleted(t *testing.T) {
	t.Parallel()
	repo := NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			return dao.NoteOfDB{Id: id, Status: domain.NoteStatusDeleted}, nil
		},
	})
	_, err := repo.FindByID(context.Background(), 3)
	if !errors.Is(err, ErrNoteDeleted) {
		t.Fatalf("want ErrNoteDeleted, got %v", err)
	}
}
