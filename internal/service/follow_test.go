package service

import (
	"context"
	"errors"
	"sync"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

type followRepoStub struct {
	createFn          func(ctx context.Context, followerID, followeeID int64) error
	deleteFn          func(ctx context.Context, followerID, followeeID int64) error
	listFollowingFn   func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error)
	listFollowersFn   func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error)
	listFolloweeIDsFn func(ctx context.Context, followerID int64, followeeIDs []int64) ([]int64, error)
}

func (s *followRepoStub) Create(ctx context.Context, followerID, followeeID int64) error {
	if s.createFn == nil {
		return nil
	}
	return s.createFn(ctx, followerID, followeeID)
}

func (s *followRepoStub) Delete(ctx context.Context, followerID, followeeID int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, followerID, followeeID)
}

func (s *followRepoStub) ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
	if s.listFollowingFn == nil {
		return nil, nil
	}
	return s.listFollowingFn(ctx, followerID, cursor, limit)
}

func (s *followRepoStub) ListFollowers(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
	if s.listFollowersFn == nil {
		return nil, nil
	}
	return s.listFollowersFn(ctx, followeeID, cursor, limit)
}

func (s *followRepoStub) ListFolloweeIDs(ctx context.Context, followerID int64, followeeIDs []int64) ([]int64, error) {
	if s.listFolloweeIDsFn == nil {
		return followeeIDs, nil
	}
	return s.listFolloweeIDsFn(ctx, followerID, followeeIDs)
}

func newFollowService(followRepo repository.FollowRepository, users repository.UserRepository) *FollowServiceImpl {
	return NewFollowServiceImpl(followRepo, users, logger.NewNoOpLogger())
}

func TestFollowService_Follow(t *testing.T) {
	t.Parallel()

	t.Run("rejects self follow", func(t *testing.T) {
		t.Parallel()
		svc := newFollowService(&followRepoStub{}, &userRepoStub{})
		if err := svc.Follow(context.Background(), 7, 7); !errors.Is(err, ErrFollowSelf) {
			t.Fatalf("want ErrFollowSelf, got %v", err)
		}
	})

	t.Run("rejects missing followee", func(t *testing.T) {
		t.Parallel()
		svc := newFollowService(&followRepoStub{}, &userRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.User, error) {
				return domain.User{}, repository.ErrUserNotFound
			},
		})
		if err := svc.Follow(context.Background(), 1, 2); !errors.Is(err, ErrFolloweeNotFound) {
			t.Fatalf("want ErrFolloweeNotFound, got %v", err)
		}
	})

	t.Run("first follow succeeds", func(t *testing.T) {
		t.Parallel()
		created := false
		svc := newFollowService(&followRepoStub{
			createFn: func(ctx context.Context, followerID, followeeID int64) error {
				created = true
				if followerID != 1 || followeeID != 2 {
					t.Fatalf("unexpected pair %d -> %d", followerID, followeeID)
				}
				return nil
			},
		}, existingUserRepo())
		if err := svc.Follow(context.Background(), 1, 2); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !created {
			t.Fatal("expected repository create")
		}
	})

	t.Run("duplicate follow is idempotent", func(t *testing.T) {
		t.Parallel()
		svc := newFollowService(&followRepoStub{
			createFn: func(ctx context.Context, followerID, followeeID int64) error {
				return repository.ErrFollowDuplicate
			},
		}, existingUserRepo())
		if err := svc.Follow(context.Background(), 1, 2); err != nil {
			t.Fatalf("duplicate follow should succeed, got %v", err)
		}
	})
}

func TestFollowService_UnfollowIsIdempotent(t *testing.T) {
	t.Parallel()
	calls := 0
	svc := newFollowService(&followRepoStub{
		deleteFn: func(ctx context.Context, followerID, followeeID int64) error {
			calls++
			return nil
		},
	}, existingUserRepo())
	if err := svc.Unfollow(context.Background(), 1, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.Unfollow(context.Background(), 1, 2); err != nil {
		t.Fatalf("repeat unfollow should succeed, got %v", err)
	}
	if calls != 2 {
		t.Fatalf("want 2 deletes, got %d", calls)
	}
}

func TestFollowService_ConcurrentDuplicateFollow(t *testing.T) {
	t.Parallel()
	repo := newMemFollowRepo()
	svc := newFollowService(repo, existingUserRepo())

	const workers = 32
	var wg sync.WaitGroup
	wg.Add(workers)
	errCh := make(chan error, workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			errCh <- svc.Follow(context.Background(), 1, 2)
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent follow returned %v", err)
		}
	}
	if repo.len() != 1 {
		t.Fatalf("want exactly 1 relation, got %d", repo.len())
	}
}

func TestFollowService_ListFollowingStableCursor(t *testing.T) {
	t.Parallel()
	sameTs := int64(1000)
	svc := newFollowService(&followRepoStub{
		listFollowingFn: func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
			all := []domain.UserRelation{
				{ID: 3, FollowerID: 1, FolloweeID: 30, CreatedAt: sameTs},
				{ID: 2, FollowerID: 1, FolloweeID: 20, CreatedAt: sameTs},
				{ID: 1, FollowerID: 1, FolloweeID: 10, CreatedAt: sameTs},
			}
			start := 0
			if cursor != nil {
				for i, row := range all {
					if row.CreatedAt < cursor.CreatedAt || (row.CreatedAt == cursor.CreatedAt && row.ID < cursor.ID) {
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
	}, existingUserRepo())

	page1, err := svc.ListFollowing(context.Background(), 1, "", 1)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page1.Items) != 1 || page1.Items[0].UserID != 30 || !page1.HasMore || page1.NextCursor == "" {
		t.Fatalf("unexpected page1: %+v", page1)
	}
	page2, err := svc.ListFollowing(context.Background(), 1, page1.NextCursor, 1)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(page2.Items) != 1 || page2.Items[0].UserID != 20 {
		t.Fatalf("cursor should continue at id=2, got %+v", page2)
	}
	if page1.Items[0].UserID == page2.Items[0].UserID {
		t.Fatal("pages overlapped")
	}
}

func TestFollowService_ListFollowersUsesFollowerIDs(t *testing.T) {
	t.Parallel()
	svc := newFollowService(&followRepoStub{
		listFollowersFn: func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
			return []domain.UserRelation{{
				ID: 9, FollowerID: 4, FolloweeID: followeeID, CreatedAt: 11,
			}}, nil
		},
	}, existingUserRepo())
	page, err := svc.ListFollowers(context.Background(), 8, "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].UserID != 4 || page.HasMore {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestFollowService_InvalidCursor(t *testing.T) {
	t.Parallel()
	svc := newFollowService(&followRepoStub{}, existingUserRepo())
	if _, err := svc.ListFollowing(context.Background(), 1, "bad", 10); !errors.Is(err, ErrInvalidCursor) {
		t.Fatalf("want ErrInvalidCursor, got %v", err)
	}
}

func existingUserRepo() *userRepoStub {
	return &userRepoStub{
		findByIDFn: func(ctx context.Context, id int64) (domain.User, error) {
			return domain.User{Id: id}, nil
		},
	}
}

type memFollowRepo struct {
	mu      sync.Mutex
	nextID  int64
	byPair  map[[2]int64]domain.UserRelation
	ordered []domain.UserRelation
}

func newMemFollowRepo() *memFollowRepo {
	return &memFollowRepo{
		byPair: make(map[[2]int64]domain.UserRelation),
	}
}

func (r *memFollowRepo) Create(ctx context.Context, followerID, followeeID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := [2]int64{followerID, followeeID}
	if _, ok := r.byPair[key]; ok {
		return repository.ErrFollowDuplicate
	}
	r.nextID++
	rel := domain.UserRelation{
		ID:         r.nextID,
		FollowerID: followerID,
		FolloweeID: followeeID,
		CreatedAt:  r.nextID,
	}
	r.byPair[key] = rel
	r.ordered = append(r.ordered, rel)
	return nil
}

func (r *memFollowRepo) Delete(ctx context.Context, followerID, followeeID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := [2]int64{followerID, followeeID}
	delete(r.byPair, key)
	return nil
}

func (r *memFollowRepo) ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
	return nil, nil
}

func (r *memFollowRepo) ListFollowers(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
	return nil, nil
}

func (r *memFollowRepo) ListFolloweeIDs(ctx context.Context, followerID int64, followeeIDs []int64) ([]int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]int64, 0, len(followeeIDs))
	for _, id := range followeeIDs {
		if _, ok := r.byPair[[2]int64{followerID, id}]; ok {
			out = append(out, id)
		}
	}
	return out, nil
}

func (r *memFollowRepo) len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.byPair)
}
