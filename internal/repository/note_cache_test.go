package repository

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
	"user-center/pkg/logger"
)

type noteCacheStub struct {
	getFn    func(ctx context.Context, id int64) (domain.Note, error)
	setFn    func(ctx context.Context, note domain.Note) error
	deleteFn func(ctx context.Context, id int64) error
}

func (s *noteCacheStub) Get(ctx context.Context, id int64) (domain.Note, error) {
	return s.getFn(ctx, id)
}

func (s *noteCacheStub) Set(ctx context.Context, note domain.Note) error {
	if s.setFn == nil {
		return nil
	}
	return s.setFn(ctx, note)
}

func (s *noteCacheStub) Delete(ctx context.Context, id int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, id)
}

func TestCachedNoteRepository_FindByIDHit(t *testing.T) {
	t.Parallel()
	want := domain.Note{ID: 9, Title: "cache hit"}
	repo := NewCachedNoteRepository(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			t.Fatal("database must not be called on cache hit")
			return dao.NoteOfDB{}, nil
		},
	}), &noteCacheStub{getFn: func(ctx context.Context, id int64) (domain.Note, error) {
		return want, nil
	}}, logger.NewNoOpLogger())

	got, err := repo.FindByID(context.Background(), want.ID)
	if err != nil || got.ID != want.ID || got.Title != want.Title {
		t.Fatalf("note=%+v err=%v", got, err)
	}
}

func TestCachedNoteRepository_FindByIDMissLoadsAndCaches(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("cache miss")
	var cached domain.Note
	repo := NewCachedNoteRepository(NewNoteRepositoryImpl(&noteDAOStub{
		findByIDFn: func(ctx context.Context, id int64) (dao.NoteOfDB, error) {
			return dao.NoteOfDB{Id: id, AuthorId: 2, Title: "database", Status: domain.NoteStatusPublished}, nil
		},
	}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, wantErr
		},
		setFn: func(ctx context.Context, note domain.Note) error {
			cached = note
			return nil
		},
	}, logger.NewNoOpLogger())

	got, err := repo.FindByID(context.Background(), 4)
	if err != nil || got.ID != 4 || cached.ID != 4 {
		t.Fatalf("note=%+v cached=%+v err=%v", got, cached, err)
	}
}

func TestCachedNoteRepository_CreateAndDeleteInvalidate(t *testing.T) {
	t.Parallel()
	var invalidated []int64
	repo := NewCachedNoteRepository(NewNoteRepositoryImpl(&noteDAOStub{
		insertFn: func(ctx context.Context, note dao.NoteOfDB) (dao.NoteOfDB, error) {
			note.Id = 6
			return note, nil
		},
	}), &noteCacheStub{
		getFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{}, errors.New("unused")
		},
		deleteFn: func(ctx context.Context, id int64) error {
			invalidated = append(invalidated, id)
			return nil
		},
	}, logger.NewNoOpLogger())

	created, err := repo.Create(context.Background(), domain.Note{AuthorID: 2})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err = repo.SoftDelete(context.Background(), created.ID, 2); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if len(invalidated) != 2 || invalidated[0] != 6 || invalidated[1] != 6 {
		t.Fatalf("invalidated=%v", invalidated)
	}
}
