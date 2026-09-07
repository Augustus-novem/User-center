package service

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository"
)

type FeedService interface {
	ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error)
}

type FeedServiceImpl struct {
	feedRepo repository.FeedRepository
}

func NewFeedServiceImpl(feedRepo repository.FeedRepository) *FeedServiceImpl {
	return &FeedServiceImpl{feedRepo: feedRepo}
}

func (s *FeedServiceImpl) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error) {
	if userID <= 0 {
		return domain.NotePage{}, ErrInvalidNoteID
	}
	cur, err := parseNoteCursor(cursor)
	if err != nil {
		return domain.NotePage{}, err
	}
	limit = normalizeNoteLimit(limit)
	rows, err := s.feedRepo.ListFollowing(ctx, userID, cur, limit+1)
	if err != nil {
		return domain.NotePage{}, err
	}
	page := domain.NotePage{Items: rows}
	if len(rows) > limit {
		page.HasMore = true
		page.Items = rows[:limit]
	}
	if page.HasMore && len(page.Items) > 0 {
		last := page.Items[len(page.Items)-1]
		page.NextCursor = encodeNoteCursor(last.CreatedAt, last.ID)
	}
	return page, nil
}
