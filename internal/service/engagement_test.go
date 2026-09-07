package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

type likeRepoStub struct {
	createFn func(ctx context.Context, userID, noteID int64) error
	deleteFn func(ctx context.Context, userID, noteID int64) error
}

func (s *likeRepoStub) Create(ctx context.Context, userID, noteID int64) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, userID, noteID)
}

func (s *likeRepoStub) Delete(ctx context.Context, userID, noteID int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, userID, noteID)
}

type commentRepoStub struct {
	createFn     func(ctx context.Context, comment domain.Comment) (domain.Comment, error)
	listByNoteFn func(ctx context.Context, noteID int64, cursor *domain.FollowCursor, limit int) ([]domain.Comment, error)
}

func (s *commentRepoStub) Create(ctx context.Context, comment domain.Comment) (domain.Comment, error) {
	if s.createFn == nil {
		comment.ID = 1
		return comment, nil
	}
	return s.createFn(ctx, comment)
}

func (s *commentRepoStub) ListByNote(ctx context.Context, noteID int64, cursor *domain.FollowCursor, limit int) ([]domain.Comment, error) {
	if s.listByNoteFn == nil {
		return nil, nil
	}
	return s.listByNoteFn(ctx, noteID, cursor, limit)
}

func publishedNoteRepo() *noteRepoStub {
	return &noteRepoStub{
		findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
			return domain.Note{ID: id, Status: domain.NoteStatusPublished}, nil
		},
	}
}

func TestEngagementService_Like(t *testing.T) {
	t.Parallel()

	t.Run("missing note", func(t *testing.T) {
		t.Parallel()
		svc := NewEngagementServiceImpl(&noteRepoStub{}, &likeRepoStub{}, &commentRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		if err := svc.Like(context.Background(), 1, 9); !errors.Is(err, ErrNoteNotFound) {
			t.Fatalf("want ErrNoteNotFound, got %v", err)
		}
	})

	t.Run("first like publishes outbox", func(t *testing.T) {
		t.Parallel()
		pub := &publisherSpy{enabled: true}
		svc := NewEngagementServiceImpl(publishedNoteRepo(), &likeRepoStub{}, &commentRepoStub{}, &txStub{}, pub, logger.NewNoOpLogger())
		if err := svc.Like(context.Background(), 2, 9); err != nil {
			t.Fatal(err)
		}
		if len(pub.calls) != 1 || pub.calls[0].topic != events.TopicNoteLiked {
			t.Fatalf("calls=%+v", pub.calls)
		}
	})

	t.Run("duplicate like is idempotent and does not emit again", func(t *testing.T) {
		t.Parallel()
		pub := &publisherSpy{enabled: true}
		svc := NewEngagementServiceImpl(publishedNoteRepo(), &likeRepoStub{
			createFn: func(ctx context.Context, userID, noteID int64) error {
				return repository.ErrLikeDuplicate
			},
		}, &commentRepoStub{}, &txStub{}, pub, logger.NewNoOpLogger())
		if err := svc.Like(context.Background(), 2, 9); err != nil {
			t.Fatalf("duplicate like should succeed: %v", err)
		}
		if len(pub.calls) != 0 {
			t.Fatalf("duplicate like must not emit event: %+v", pub.calls)
		}
	})

	t.Run("concurrent likes keep one row", func(t *testing.T) {
		t.Parallel()
		store := newMemLikeStore()
		svc := NewEngagementServiceImpl(publishedNoteRepo(), store, &commentRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		var wg sync.WaitGroup
		wg.Add(20)
		for i := 0; i < 20; i++ {
			go func() {
				defer wg.Done()
				if err := svc.Like(context.Background(), 1, 2); err != nil {
					t.Errorf("like: %v", err)
				}
			}()
		}
		wg.Wait()
		if store.len() != 1 {
			t.Fatalf("want 1 like, got %d", store.len())
		}
	})
}

func TestEngagementService_UnlikeAndComment(t *testing.T) {
	t.Parallel()

	t.Run("unlike is idempotent", func(t *testing.T) {
		t.Parallel()
		calls := 0
		svc := NewEngagementServiceImpl(publishedNoteRepo(), &likeRepoStub{
			deleteFn: func(ctx context.Context, userID, noteID int64) error {
				calls++
				return nil
			},
		}, &commentRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		_ = svc.Unlike(context.Background(), 1, 2)
		_ = svc.Unlike(context.Background(), 1, 2)
		if calls != 2 {
			t.Fatalf("calls=%d", calls)
		}
	})

	t.Run("empty comment", func(t *testing.T) {
		t.Parallel()
		svc := NewEngagementServiceImpl(publishedNoteRepo(), &likeRepoStub{}, &commentRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		if _, err := svc.CreateComment(context.Background(), 1, 2, "   "); !errors.Is(err, ErrInvalidComment) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("comment too long", func(t *testing.T) {
		t.Parallel()
		svc := NewEngagementServiceImpl(publishedNoteRepo(), &likeRepoStub{}, &commentRepoStub{}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		if _, err := svc.CreateComment(context.Background(), 1, 2, strings.Repeat("啊", 501)); !errors.Is(err, ErrInvalidComment) {
			t.Fatalf("got %v", err)
		}
	})

	t.Run("comment cursor is stable", func(t *testing.T) {
		t.Parallel()
		svc := NewEngagementServiceImpl(publishedNoteRepo(), &likeRepoStub{}, &commentRepoStub{
			listByNoteFn: func(ctx context.Context, noteID int64, cursor *domain.FollowCursor, limit int) ([]domain.Comment, error) {
				all := []domain.Comment{
					{ID: 1, Content: "a", CreatedAt: 10},
					{ID: 2, Content: "b", CreatedAt: 10},
					{ID: 3, Content: "c", CreatedAt: 11},
				}
				start := 0
				if cursor != nil {
					for i, row := range all {
						if row.CreatedAt > cursor.CreatedAt || (row.CreatedAt == cursor.CreatedAt && row.ID > cursor.ID) {
							start = i
							break
						}
						if i == len(all)-1 {
							start = len(all)
						}
					}
				}
				end := start + limit
				if end > len(all) {
					end = len(all)
				}
				return all[start:end], nil
			},
		}, &txStub{}, events.NopPublisher{}, logger.NewNoOpLogger())
		page1, err := svc.ListComments(context.Background(), 9, "", 1)
		if err != nil || page1.Items[0].Content != "a" || !page1.HasMore {
			t.Fatalf("page1=%+v err=%v", page1, err)
		}
		page2, err := svc.ListComments(context.Background(), 9, page1.NextCursor, 1)
		if err != nil || page2.Items[0].Content != "b" {
			t.Fatalf("page2=%+v err=%v", page2, err)
		}
	})
}

type memLikeStore struct {
	mu   sync.Mutex
	rows map[[2]int64]struct{}
}

func newMemLikeStore() *memLikeStore {
	return &memLikeStore{rows: make(map[[2]int64]struct{})}
}

func (s *memLikeStore) Create(ctx context.Context, userID, noteID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key := [2]int64{userID, noteID}
	if _, ok := s.rows[key]; ok {
		return repository.ErrLikeDuplicate
	}
	s.rows[key] = struct{}{}
	return nil
}

func (s *memLikeStore) Delete(ctx context.Context, userID, noteID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.rows, [2]int64{userID, noteID})
	return nil
}

func (s *memLikeStore) len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.rows)
}
