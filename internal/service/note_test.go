package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

type noteRepoStub struct {
	createFn       func(ctx context.Context, note domain.Note) (domain.Note, error)
	findByIDFn     func(ctx context.Context, id int64) (domain.Note, error)
	listByAuthorFn func(ctx context.Context, authorID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error)
	softDeleteFn   func(ctx context.Context, id, authorID int64) error
}

func (s *noteRepoStub) Create(ctx context.Context, note domain.Note) (domain.Note, error) {
	if s.createFn == nil {
		note.ID = 1
		return note, nil
	}
	return s.createFn(ctx, note)
}

func (s *noteRepoStub) FindByID(ctx context.Context, id int64) (domain.Note, error) {
	if s.findByIDFn == nil {
		return domain.Note{}, repository.ErrNoteNotFound
	}
	return s.findByIDFn(ctx, id)
}

func (s *noteRepoStub) ListByAuthor(ctx context.Context, authorID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
	if s.listByAuthorFn == nil {
		return nil, nil
	}
	return s.listByAuthorFn(ctx, authorID, cursor, limit)
}

func (s *noteRepoStub) SoftDelete(ctx context.Context, id, authorID int64) error {
	if s.softDeleteFn == nil {
		return nil
	}
	return s.softDeleteFn(ctx, id, authorID)
}

func TestNoteService_Publish(t *testing.T) {
	t.Parallel()

	t.Run("writes note images and outbox in one transaction", func(t *testing.T) {
		t.Parallel()
		inTx := false
		publisher := &publisherSpy{enabled: true}
		repo := &noteRepoStub{
			createFn: func(ctx context.Context, note domain.Note) (domain.Note, error) {
				if !inTx {
					t.Fatal("Create must run inside transaction")
				}
				if len(note.Images) != 2 || note.Images[0].URL != "https://a" || note.Images[1].SortOrder != 1 {
					t.Fatalf("image order: %+v", note.Images)
				}
				note.ID = 42
				return note, nil
			},
		}
		svc := NewNoteServiceImpl(repo, &txStub{inTxFn: func(ctx context.Context, fn func(context.Context) error) error {
			inTx = true
			return fn(ctx)
		}}, publisher, logger.NewNoOpLogger())

		note, err := svc.Publish(context.Background(), 9, "hello", "body", []string{"https://a", "https://b"})
		if err != nil {
			t.Fatalf("publish: %v", err)
		}
		if note.ID != 42 {
			t.Fatalf("unexpected note: %+v", note)
		}
		if len(publisher.calls) != 1 || publisher.calls[0].topic != events.TopicNotePublished {
			t.Fatalf("unexpected publish calls: %+v", publisher.calls)
		}
		evt, ok := publisher.calls[0].value.(events.NotePublishedEvent)
		if !ok || evt.NoteID != 42 || evt.AuthorID != 9 || evt.Type != events.TopicNotePublished || evt.EventID == "" {
			t.Fatalf("unexpected event: %+v", publisher.calls[0].value)
		}
	})

	t.Run("rolls back note when outbox publish fails", func(t *testing.T) {
		t.Parallel()
		store := newMemNoteStore()
		publisher := &publisherSpy{enabled: true, fn: func(ctx context.Context, topic, key string, value any) error {
			return errors.New("outbox down")
		}}
		svc := NewNoteServiceImpl(store, store, publisher, logger.NewNoOpLogger())
		_, err := svc.Publish(context.Background(), 1, "hello", "body", []string{"https://a"})
		if err == nil {
			t.Fatal("expected outbox failure")
		}
		if store.len() != 0 {
			t.Fatalf("note should roll back, got %d", store.len())
		}
	})

	t.Run("rejects empty title", func(t *testing.T) {
		t.Parallel()
		svc := NewNoteServiceImpl(&noteRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		if _, err := svc.Publish(context.Background(), 1, "  ", "body", nil); !errors.Is(err, ErrInvalidNote) {
			t.Fatalf("want ErrInvalidNote, got %v", err)
		}
	})
}

func TestNoteService_Delete(t *testing.T) {
	t.Parallel()

	t.Run("author can delete", func(t *testing.T) {
		t.Parallel()
		deleted := false
		svc := NewNoteServiceImpl(&noteRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
				return domain.Note{ID: id, AuthorID: 7, Status: domain.NoteStatusPublished}, nil
			},
			softDeleteFn: func(ctx context.Context, id, authorID int64) error {
				deleted = true
				return nil
			},
		}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		if err := svc.Delete(context.Background(), 7, 3); err != nil {
			t.Fatalf("delete: %v", err)
		}
		if !deleted {
			t.Fatal("expected soft delete")
		}
	})

	t.Run("non-author cannot delete", func(t *testing.T) {
		t.Parallel()
		svc := NewNoteServiceImpl(&noteRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
				return domain.Note{ID: id, AuthorID: 7}, nil
			},
			softDeleteFn: func(ctx context.Context, id, authorID int64) error {
				t.Fatal("should not delete")
				return nil
			},
		}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		if err := svc.Delete(context.Background(), 8, 3); !errors.Is(err, ErrNoteForbidden) {
			t.Fatalf("want ErrNoteForbidden, got %v", err)
		}
	})
}

type memNoteStore struct {
	mu     sync.Mutex
	nextID int64
	notes  map[int64]domain.Note
}

func newMemNoteStore() *memNoteStore {
	return &memNoteStore{notes: make(map[int64]domain.Note)}
}

func (s *memNoteStore) InTx(ctx context.Context, fn func(context.Context) error) error {
	s.mu.Lock()
	snap := cloneNotes(s.notes)
	s.mu.Unlock()
	err := fn(ctx)
	if err != nil {
		s.mu.Lock()
		s.notes = snap
		s.mu.Unlock()
	}
	return err
}

func (s *memNoteStore) Create(ctx context.Context, note domain.Note) (domain.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextID++
	note.ID = s.nextID
	s.notes[note.ID] = note
	return note, nil
}

func (s *memNoteStore) FindByID(ctx context.Context, id int64) (domain.Note, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	note, ok := s.notes[id]
	if !ok {
		return domain.Note{}, repository.ErrNoteNotFound
	}
	return note, nil
}

func (s *memNoteStore) ListByAuthor(ctx context.Context, authorID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
	return nil, nil
}

func (s *memNoteStore) SoftDelete(ctx context.Context, id, authorID int64) error {
	return nil
}

func (s *memNoteStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.notes)
}

func cloneNotes(in map[int64]domain.Note) map[int64]domain.Note {
	out := make(map[int64]domain.Note, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
