package repository

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

type followDAOStub struct {
	insertFn        func(ctx context.Context, rel dao.UserRelationOfDB) error
	deleteFn        func(ctx context.Context, followerID, followeeID int64) error
	listFollowingFn func(ctx context.Context, followerID int64, cursor *dao.FollowCursor, limit int) ([]dao.UserRelationOfDB, error)
	listFollowersFn func(ctx context.Context, followeeID int64, cursor *dao.FollowCursor, limit int) ([]dao.UserRelationOfDB, error)
}

func (s *followDAOStub) Insert(ctx context.Context, rel dao.UserRelationOfDB) error {
	if s.insertFn == nil {
		return nil
	}
	return s.insertFn(ctx, rel)
}

func (s *followDAOStub) Delete(ctx context.Context, followerID, followeeID int64) error {
	if s.deleteFn == nil {
		return nil
	}
	return s.deleteFn(ctx, followerID, followeeID)
}

func (s *followDAOStub) ListFollowing(ctx context.Context, followerID int64, cursor *dao.FollowCursor, limit int) ([]dao.UserRelationOfDB, error) {
	if s.listFollowingFn == nil {
		return nil, nil
	}
	return s.listFollowingFn(ctx, followerID, cursor, limit)
}

func (s *followDAOStub) ListFollowers(ctx context.Context, followeeID int64, cursor *dao.FollowCursor, limit int) ([]dao.UserRelationOfDB, error) {
	if s.listFollowersFn == nil {
		return nil, nil
	}
	return s.listFollowersFn(ctx, followeeID, cursor, limit)
}

func TestFollowRepositoryImpl_CreateMapsDuplicate(t *testing.T) {
	t.Parallel()
	repo := NewFollowRepositoryImpl(&followDAOStub{
		insertFn: func(ctx context.Context, rel dao.UserRelationOfDB) error {
			if rel.FollowerId != 1 || rel.FolloweeId != 2 {
				t.Fatalf("unexpected relation: %+v", rel)
			}
			return dao.ErrFollowDuplicate
		},
	})
	err := repo.Create(context.Background(), 1, 2)
	if !errors.Is(err, ErrFollowDuplicate) {
		t.Fatalf("want ErrFollowDuplicate, got %v", err)
	}
}

func TestFollowRepositoryImpl_ListFollowingMapsCursorAndRows(t *testing.T) {
	t.Parallel()
	repo := NewFollowRepositoryImpl(&followDAOStub{
		listFollowingFn: func(ctx context.Context, followerID int64, cursor *dao.FollowCursor, limit int) ([]dao.UserRelationOfDB, error) {
			if followerID != 9 || limit != 3 || cursor == nil || cursor.CreatedAt != 50 || cursor.ID != 4 {
				t.Fatalf("unexpected args follower=%d cursor=%+v limit=%d", followerID, cursor, limit)
			}
			return []dao.UserRelationOfDB{{
				Id: 3, FollowerId: 9, FolloweeId: 8, CreatedAt: 40,
			}}, nil
		},
	})
	rows, err := repo.ListFollowing(context.Background(), 9, &domain.FollowCursor{CreatedAt: 50, ID: 4}, 3)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 1 || rows[0].FolloweeID != 8 || rows[0].ID != 3 {
		t.Fatalf("unexpected rows: %+v", rows)
	}
}
