package service

import (
	"context"
	"fmt"
	"testing"
	"user-center/internal/domain"
	"user-center/pkg/logger"
)

func BenchmarkFeedStrategies(b *testing.B) {
	for _, followers := range []int{100, 1000, 10000} {
		followers := followers
		b.Run(fmt.Sprintf("publish_push_followers_%d", followers), func(b *testing.B) {
			svc := NewFeedServiceImpl(newMemFeedInbox(), &noteRepoStub{}, benchmarkFollowers(followers), 200, 0, logger.NewNoOpLogger())
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := svc.FanoutPublished(context.Background(), 1, 99); err != nil {
					b.Fatal(err)
				}
			}
		})
		b.Run(fmt.Sprintf("publish_pull_followers_%d", followers), func(b *testing.B) {
			follows := benchmarkFollowers(followers)
			follows.countFollowersFn = func(context.Context, int64) (int64, error) { return int64(followers), nil }
			svc := NewFeedServiceImpl(newMemFeedInbox(), &noteRepoStub{}, follows, 200, 1, logger.NewNoOpLogger())
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				if err := svc.FanoutPublished(context.Background(), 1, 99); err != nil {
					b.Fatal(err)
				}
			}
		})
	}

	b.Run("read_push_top20", func(b *testing.B) {
		inbox := newMemFeedInbox()
		ids := make([]int64, 20)
		for i := range ids {
			ids[i] = int64(i + 1)
		}
		_ = inbox.AddToUsers(context.Background(), []int64{1}, ids[0])
		for _, id := range ids[1:] {
			_ = inbox.AddToUsers(context.Background(), []int64{1}, id)
		}
		notes := &noteRepoStub{findByIDFn: func(_ context.Context, id int64) (domain.Note, error) {
			return domain.Note{ID: id, AuthorID: 8, Status: domain.NoteStatusPublished}, nil
		}}
		follows := &followRepoStub{listFolloweeIDsFn: func(_ context.Context, _ int64, followeeIDs []int64) ([]int64, error) {
			return followeeIDs, nil
		}}
		svc := NewFeedServiceImpl(inbox, notes, follows, 200, 0, logger.NewNoOpLogger())
		benchmarkFeedRead(b, svc)
	})

	b.Run("read_pull_top20", func(b *testing.B) {
		notes := &noteRepoStub{listPublishedBeforeFn: func(_ context.Context, authorID, exclusiveMaxID int64, limit int) ([]domain.Note, error) {
			return benchmarkNotes(authorID, exclusiveMaxID, limit, 40), nil
		}}
		follows := &followRepoStub{
			listFollowingFn: func(_ context.Context, _ int64, cursor *domain.FollowCursor, _ int) ([]domain.UserRelation, error) {
				if cursor != nil {
					return nil, nil
				}
				return []domain.UserRelation{{ID: 1, FollowerID: 1, FolloweeID: 9}}, nil
			},
			filterIDsByMinFollowersFn: func(_ context.Context, _ []int64, _ int) ([]int64, error) {
				return []int64{9}, nil
			},
		}
		svc := NewFeedServiceImpl(newMemFeedInbox(), notes, follows, 200, 1, logger.NewNoOpLogger())
		benchmarkFeedRead(b, svc)
	})

	b.Run("read_hybrid_top20", func(b *testing.B) {
		inbox := newMemFeedInbox()
		for id := int64(21); id <= 30; id++ {
			_ = inbox.AddToUsers(context.Background(), []int64{1}, id)
		}
		notes := &noteRepoStub{
			findByIDFn: func(_ context.Context, id int64) (domain.Note, error) {
				return domain.Note{ID: id, AuthorID: 8, Status: domain.NoteStatusPublished}, nil
			},
			listPublishedBeforeFn: func(_ context.Context, authorID, exclusiveMaxID int64, limit int) ([]domain.Note, error) {
				return benchmarkNotes(authorID, exclusiveMaxID, limit, 20), nil
			},
		}
		follows := &followRepoStub{
			listFollowingFn: func(_ context.Context, _ int64, cursor *domain.FollowCursor, _ int) ([]domain.UserRelation, error) {
				if cursor != nil {
					return nil, nil
				}
				return []domain.UserRelation{{ID: 2, FollowerID: 1, FolloweeID: 9}, {ID: 1, FollowerID: 1, FolloweeID: 8}}, nil
			},
			filterIDsByMinFollowersFn: func(_ context.Context, _ []int64, _ int) ([]int64, error) {
				return []int64{9}, nil
			},
			listFolloweeIDsFn: func(_ context.Context, _ int64, ids []int64) ([]int64, error) { return ids, nil },
		}
		svc := NewFeedServiceImpl(inbox, notes, follows, 200, 1000, logger.NewNoOpLogger())
		benchmarkFeedRead(b, svc)
	})
}

func benchmarkFollowers(count int) *followRepoStub {
	rows := make([]domain.UserRelation, count)
	for i := range rows {
		id := int64(count - i)
		rows[i] = domain.UserRelation{ID: id, FollowerID: int64(i + 1), FolloweeID: 99, CreatedAt: id}
	}
	return &followRepoStub{listFollowersFn: func(_ context.Context, _ int64, cursor *domain.FollowCursor, limit int) ([]domain.UserRelation, error) {
		start := 0
		if cursor != nil {
			start = count
			for i, row := range rows {
				if row.ID < cursor.ID {
					start = i
					break
				}
			}
		}
		end := start + limit
		if end > count {
			end = count
		}
		return rows[start:end], nil
	}}
}

func benchmarkNotes(authorID, exclusiveMaxID int64, limit, maxID int) []domain.Note {
	rows := make([]domain.Note, 0, limit)
	for id := int64(maxID); id > 0 && len(rows) < limit; id-- {
		if exclusiveMaxID > 0 && id >= exclusiveMaxID {
			continue
		}
		rows = append(rows, domain.Note{ID: id, AuthorID: authorID, Status: domain.NoteStatusPublished})
	}
	return rows
}

func benchmarkFeedRead(b *testing.B, svc *FeedServiceImpl) {
	b.Helper()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		page, err := svc.ListFollowing(context.Background(), 1, "", 20)
		if err != nil || len(page.Items) != 20 {
			b.Fatalf("items=%d err=%v", len(page.Items), err)
		}
	}
}
