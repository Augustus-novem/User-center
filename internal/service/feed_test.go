package service

import (
	"context"
	"testing"
	"user-center/internal/domain"
)

type feedRepoStub struct {
	listFn func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error)
}

func (s *feedRepoStub) ListFollowing(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
	if s.listFn == nil {
		return nil, nil
	}
	return s.listFn(ctx, followerID, cursor, limit)
}

func TestFeedService_ListFollowing(t *testing.T) {
	t.Parallel()

	t.Run("empty following", func(t *testing.T) {
		t.Parallel()
		svc := NewFeedServiceImpl(&feedRepoStub{
			listFn: func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
				return []domain.Note{}, nil
			},
		})
		page, err := svc.ListFollowing(context.Background(), 1, "", 10)
		if err != nil || len(page.Items) != 0 || page.HasMore {
			t.Fatalf("page=%+v err=%v", page, err)
		}
	})

	t.Run("stable cursor across same created_at", func(t *testing.T) {
		t.Parallel()
		all := []domain.Note{
			{ID: 30, AuthorID: 8, Title: "c", CreatedAt: 100, Status: domain.NoteStatusPublished},
			{ID: 20, AuthorID: 7, Title: "b", CreatedAt: 100, Status: domain.NoteStatusPublished},
			{ID: 10, AuthorID: 8, Title: "a", CreatedAt: 90, Status: domain.NoteStatusPublished},
		}
		svc := NewFeedServiceImpl(&feedRepoStub{
			listFn: func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
				start := 0
				if cursor != nil {
					for i, n := range all {
						if n.CreatedAt < cursor.CreatedAt || (n.CreatedAt == cursor.CreatedAt && n.ID < cursor.ID) {
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
		})
		page1, err := svc.ListFollowing(context.Background(), 1, "", 1)
		if err != nil || page1.Items[0].ID != 30 || !page1.HasMore {
			t.Fatalf("page1=%+v err=%v", page1, err)
		}
		page2, err := svc.ListFollowing(context.Background(), 1, page1.NextCursor, 1)
		if err != nil || page2.Items[0].ID != 20 {
			t.Fatalf("page2=%+v err=%v", page2, err)
		}
	})

	t.Run("new note inserted between pages is not duplicated", func(t *testing.T) {
		t.Parallel()
		notes := []domain.Note{
			{ID: 2, AuthorID: 3, CreatedAt: 200, Status: domain.NoteStatusPublished},
			{ID: 1, AuthorID: 4, CreatedAt: 100, Status: domain.NoteStatusPublished},
		}
		svc := NewFeedServiceImpl(&feedRepoStub{
			listFn: func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
				start := 0
				if cursor != nil {
					for i, n := range notes {
						if n.CreatedAt < cursor.CreatedAt || (n.CreatedAt == cursor.CreatedAt && n.ID < cursor.ID) {
							start = i
							break
						}
						if i == len(notes)-1 {
							start = len(notes)
						}
					}
				}
				end := start + limit
				if end > len(notes) {
					end = len(notes)
				}
				return notes[start:end], nil
			},
		})
		page1, err := svc.ListFollowing(context.Background(), 1, "", 1)
		if err != nil || page1.Items[0].ID != 2 {
			t.Fatalf("page1=%+v err=%v", page1, err)
		}
		// Newer note appears after page1 was fetched. It must not show on page2.
		notes = append([]domain.Note{{ID: 3, AuthorID: 3, CreatedAt: 300, Status: domain.NoteStatusPublished}}, notes...)
		page2, err := svc.ListFollowing(context.Background(), 1, page1.NextCursor, 1)
		if err != nil || page2.Items[0].ID != 1 {
			t.Fatalf("page2 should continue older notes, got %+v err=%v", page2, err)
		}
		if page2.Items[0].ID == 3 || page2.Items[0].ID == 2 {
			t.Fatal("inserted/new or previous page item leaked into page2")
		}
	})

	t.Run("deleted notes are not returned by repository contract", func(t *testing.T) {
		t.Parallel()
		svc := NewFeedServiceImpl(&feedRepoStub{
			listFn: func(ctx context.Context, followerID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
				return []domain.Note{{ID: 1, Status: domain.NoteStatusPublished}}, nil
			},
		})
		page, err := svc.ListFollowing(context.Background(), 1, "", 10)
		if err != nil || len(page.Items) != 1 || page.Items[0].Status == domain.NoteStatusDeleted {
			t.Fatalf("page=%+v err=%v", page, err)
		}
	})
}
