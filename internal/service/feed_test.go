package service

import (
	"context"
	"errors"
	"sort"
	"sync"
	"testing"
	"user-center/internal/domain"
	"user-center/pkg/logger"
)

type memFeedInbox struct {
	mu       sync.Mutex
	members  map[int64]map[int64]struct{}
	maxItems int
	calls    int
	failNth  int
}

func newMemFeedInbox() *memFeedInbox {
	return &memFeedInbox{members: make(map[int64]map[int64]struct{})}
}

func (m *memFeedInbox) AddToUsers(ctx context.Context, userIDs []int64, noteID int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	if m.failNth > 0 && m.calls == m.failNth {
		return errors.New("inbox write failed")
	}
	for _, userID := range userIDs {
		set, ok := m.members[userID]
		if !ok {
			set = make(map[int64]struct{})
			m.members[userID] = set
		}
		set[noteID] = struct{}{}
		if m.maxItems > 0 {
			ids := sortedNoteIDs(set)
			for len(ids) > m.maxItems {
				delete(set, ids[0])
				ids = ids[1:]
			}
		}
	}
	return nil
}

func (m *memFeedInbox) List(ctx context.Context, userID int64, exclusiveMaxNoteID int64, limit int) ([]int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ids := sortedNoteIDs(m.members[userID])
	for i, j := 0, len(ids)-1; i < j; i, j = i+1, j-1 {
		ids[i], ids[j] = ids[j], ids[i]
	}
	out := make([]int64, 0, limit)
	for _, id := range ids {
		if exclusiveMaxNoteID > 0 && id >= exclusiveMaxNoteID {
			continue
		}
		out = append(out, id)
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (m *memFeedInbox) noteIDs(userID int64) []int64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return sortedNoteIDs(m.members[userID])
}

func sortedNoteIDs(set map[int64]struct{}) []int64 {
	ids := make([]int64, 0, len(set))
	for id := range set {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func newTestFeedService(inbox *memFeedInbox, notes *noteRepoStub, follows *followRepoStub, batch int) *FeedServiceImpl {
	return NewFeedServiceImpl(inbox, notes, follows, batch, logger.NewNoOpLogger())
}

func TestFeedService_FanoutPublished(t *testing.T) {
	t.Parallel()

	t.Run("zero followers does not write inbox", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		svc := newTestFeedService(inbox, &noteRepoStub{}, &followRepoStub{
			listFollowersFn: func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
				return nil, nil
			},
		}, 2)
		if err := svc.FanoutPublished(context.Background(), 10, 1); err != nil {
			t.Fatalf("FanoutPublished: %v", err)
		}
		if inbox.calls != 0 {
			t.Fatalf("want no inbox writes, calls=%d", inbox.calls)
		}
	})

	t.Run("one follower", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		svc := newTestFeedService(inbox, &noteRepoStub{}, &followRepoStub{
			listFollowersFn: func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
				if cursor != nil {
					return nil, nil
				}
				return []domain.UserRelation{{ID: 1, FollowerID: 7, FolloweeID: followeeID, CreatedAt: 100}}, nil
			},
		}, 200)
		if err := svc.FanoutPublished(context.Background(), 99, 3); err != nil {
			t.Fatalf("FanoutPublished: %v", err)
		}
		got := inbox.noteIDs(7)
		if len(got) != 1 || got[0] != 99 {
			t.Fatalf("inbox=%v", got)
		}
	})

	t.Run("multiple fanout batches", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		followers := []domain.UserRelation{
			{ID: 3, FollowerID: 31, FolloweeID: 1, CreatedAt: 30},
			{ID: 2, FollowerID: 21, FolloweeID: 1, CreatedAt: 20},
			{ID: 1, FollowerID: 11, FolloweeID: 1, CreatedAt: 10},
		}
		svc := newTestFeedService(inbox, &noteRepoStub{}, &followRepoStub{
			listFollowersFn: func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
				start := 0
				if cursor != nil {
					for i, rel := range followers {
						if rel.CreatedAt < cursor.CreatedAt || (rel.CreatedAt == cursor.CreatedAt && rel.ID < cursor.ID) {
							start = i
							break
						}
						if i == len(followers)-1 {
							start = len(followers)
						}
					}
				}
				end := start + limit
				if end > len(followers) {
					end = len(followers)
				}
				return followers[start:end], nil
			},
		}, 2)
		if err := svc.FanoutPublished(context.Background(), 5, 1); err != nil {
			t.Fatalf("FanoutPublished: %v", err)
		}
		if inbox.calls != 2 {
			t.Fatalf("want 2 batches, calls=%d", inbox.calls)
		}
		for _, uid := range []int64{31, 21, 11} {
			if got := inbox.noteIDs(uid); len(got) != 1 || got[0] != 5 {
				t.Fatalf("user %d inbox=%v", uid, got)
			}
		}
	})

	t.Run("partial batch failure then retry does not duplicate", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		inbox.failNth = 2
		followers := []domain.UserRelation{
			{ID: 2, FollowerID: 21, FolloweeID: 1, CreatedAt: 20},
			{ID: 1, FollowerID: 11, FolloweeID: 1, CreatedAt: 10},
		}
		follows := &followRepoStub{
			listFollowersFn: func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
				start := 0
				if cursor != nil {
					start = len(followers)
					for i, rel := range followers {
						if rel.CreatedAt < cursor.CreatedAt || (rel.CreatedAt == cursor.CreatedAt && rel.ID < cursor.ID) {
							start = i
							break
						}
					}
				}
				end := start + limit
				if end > len(followers) {
					end = len(followers)
				}
				return followers[start:end], nil
			},
		}
		svc := newTestFeedService(inbox, &noteRepoStub{}, follows, 1)
		if err := svc.FanoutPublished(context.Background(), 8, 1); err == nil {
			t.Fatal("want second batch error")
		}
		if got := inbox.noteIDs(21); len(got) != 1 || got[0] != 8 {
			t.Fatalf("first batch should persist, got %v", inbox.noteIDs(21))
		}
		if got := inbox.noteIDs(11); len(got) != 0 {
			t.Fatalf("second batch should not persist, got %v", got)
		}
		if err := svc.FanoutPublished(context.Background(), 8, 1); err != nil {
			t.Fatalf("retry: %v", err)
		}
		if got := inbox.noteIDs(21); len(got) != 1 || got[0] != 8 {
			t.Fatalf("retry must not duplicate, inbox=%v", got)
		}
		if got := inbox.noteIDs(11); len(got) != 1 || got[0] != 8 {
			t.Fatalf("retry must fill remaining follower, inbox=%v", got)
		}
	})

	t.Run("inbox trim keeps newest note ids", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		inbox.maxItems = 2
		svc := newTestFeedService(inbox, &noteRepoStub{}, &followRepoStub{
			listFollowersFn: func(ctx context.Context, followeeID int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
				if cursor != nil {
					return nil, nil
				}
				return []domain.UserRelation{{ID: 1, FollowerID: 7, FolloweeID: followeeID}}, nil
			},
		}, 10)
		for _, noteID := range []int64{1, 2, 3} {
			if err := svc.FanoutPublished(context.Background(), noteID, 1); err != nil {
				t.Fatalf("fanout %d: %v", noteID, err)
			}
		}
		got := inbox.noteIDs(7)
		if len(got) != 2 || got[0] != 2 || got[1] != 3 {
			t.Fatalf("trimmed inbox=%v", got)
		}
	})
}

func TestFeedService_ListFollowingFromInbox(t *testing.T) {
	t.Parallel()

	published := func(id, author int64) domain.Note {
		return domain.Note{ID: id, AuthorID: author, Title: "n", Status: domain.NoteStatusPublished}
	}

	t.Run("empty inbox", func(t *testing.T) {
		t.Parallel()
		svc := newTestFeedService(newMemFeedInbox(), &noteRepoStub{}, &followRepoStub{}, 20)
		page, err := svc.ListFollowing(context.Background(), 1, "", 10)
		if err != nil || len(page.Items) != 0 || page.HasMore {
			t.Fatalf("page=%+v err=%v", page, err)
		}
	})

	t.Run("stable cursor by note_id", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 30)
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 20)
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 10)
		notes := &noteRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
				switch id {
				case 30:
					return published(30, 8), nil
				case 20:
					return published(20, 7), nil
				case 10:
					return published(10, 8), nil
				default:
					return domain.Note{}, errors.New("missing")
				}
			},
		}
		svc := newTestFeedService(inbox, notes, &followRepoStub{}, 20)
		page1, err := svc.ListFollowing(context.Background(), 1, "", 1)
		if err != nil || len(page1.Items) != 1 || page1.Items[0].ID != 30 || !page1.HasMore {
			t.Fatalf("page1=%+v err=%v", page1, err)
		}
		page2, err := svc.ListFollowing(context.Background(), 1, page1.NextCursor, 1)
		if err != nil || len(page2.Items) != 1 || page2.Items[0].ID != 20 {
			t.Fatalf("page2=%+v err=%v", page2, err)
		}
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 40)
		page3, err := svc.ListFollowing(context.Background(), 1, page2.NextCursor, 1)
		if err != nil || len(page3.Items) != 1 || page3.Items[0].ID != 10 {
			t.Fatalf("page3 should continue older notes, got %+v err=%v", page3, err)
		}
	})

	t.Run("deleted unpublished and missing candidates are skipped", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 4)
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 3)
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 2)
		notes := &noteRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
				switch id {
				case 4:
					return domain.Note{ID: 4, AuthorID: 8, Status: domain.NoteStatusDeleted}, nil
				case 2:
					return published(2, 8), nil
				default:
					return domain.Note{}, errors.New("missing")
				}
			},
		}
		svc := newTestFeedService(inbox, notes, &followRepoStub{}, 20)
		page, err := svc.ListFollowing(context.Background(), 1, "", 10)
		if err != nil {
			t.Fatalf("ListFollowing: %v", err)
		}
		if len(page.Items) != 1 || page.Items[0].ID != 2 {
			t.Fatalf("want only published note 2, got %+v", page)
		}
	})

	t.Run("unfollow stale candidate is skipped", func(t *testing.T) {
		t.Parallel()
		inbox := newMemFeedInbox()
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 9)
		_ = inbox.AddToUsers(context.Background(), []int64{1}, 8)
		notes := &noteRepoStub{
			findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
				if id == 9 {
					return published(9, 100), nil
				}
				return published(8, 200), nil
			},
		}
		svc := newTestFeedService(inbox, notes, &followRepoStub{
			listFolloweeIDsFn: func(ctx context.Context, followerID int64, followeeIDs []int64) ([]int64, error) {
				keep := make([]int64, 0, len(followeeIDs))
				for _, id := range followeeIDs {
					if id == 200 {
						keep = append(keep, id)
					}
				}
				return keep, nil
			},
		}, 20)
		page, err := svc.ListFollowing(context.Background(), 1, "", 10)
		if err != nil || len(page.Items) != 1 || page.Items[0].ID != 8 {
			t.Fatalf("want remaining followee note, got %+v err=%v", page, err)
		}
	})
}
