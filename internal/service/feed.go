package service

import (
	"context"
	"strconv"
	"user-center/internal/domain"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

const maxFeedHydrateRounds = 20

type FeedService interface {
	ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error)
	FanoutPublished(ctx context.Context, noteID, authorID int64) error
}

type FeedServiceImpl struct {
	inbox       repository.FeedInbox
	notes       repository.NoteRepository
	follows     repository.FollowRepository
	fanoutBatch int
	logger      logger.Logger
}

func NewFeedServiceImpl(
	inbox repository.FeedInbox,
	notes repository.NoteRepository,
	follows repository.FollowRepository,
	fanoutBatch int,
	l logger.Logger,
) *FeedServiceImpl {
	if fanoutBatch <= 0 {
		fanoutBatch = 200
	}
	if l == nil {
		l = logger.NewNoOpLogger()
	}
	return &FeedServiceImpl{
		inbox:       inbox,
		notes:       notes,
		follows:     follows,
		fanoutBatch: fanoutBatch,
		logger:      l,
	}
}

func (s *FeedServiceImpl) FanoutPublished(ctx context.Context, noteID, authorID int64) error {
	if noteID <= 0 || authorID <= 0 {
		return ErrInvalidNoteID
	}
	var cursor *domain.FollowCursor
	for {
		batch, err := s.follows.ListFollowers(ctx, authorID, cursor, s.fanoutBatch)
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		userIDs := make([]int64, 0, len(batch))
		for _, rel := range batch {
			userIDs = append(userIDs, rel.FollowerID)
		}
		if err = s.inbox.AddToUsers(ctx, userIDs, noteID); err != nil {
			return err
		}
		if len(batch) < s.fanoutBatch {
			return nil
		}
		last := batch[len(batch)-1]
		cursor = &domain.FollowCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
}

func (s *FeedServiceImpl) ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error) {
	if userID <= 0 {
		return domain.NotePage{}, ErrInvalidNoteID
	}
	exclusiveMax, err := parseFeedCursor(cursor)
	if err != nil {
		return domain.NotePage{}, err
	}
	limit = normalizeNoteLimit(limit)
	need := limit + 1
	items := make([]domain.Note, 0, need)
	readBatch := need
	if readBatch < 20 {
		readBatch = 20
	}
	for round := 0; round < maxFeedHydrateRounds && len(items) < need; round++ {
		ids, err := s.inbox.List(ctx, userID, exclusiveMax, readBatch)
		if err != nil {
			return domain.NotePage{}, err
		}
		if len(ids) == 0 {
			break
		}
		notes, err := s.notes.FindByIDs(ctx, ids)
		if err != nil {
			return domain.NotePage{}, err
		}
		authors := make([]int64, 0, len(notes))
		for _, id := range ids {
			note, ok := notes[id]
			if !ok || note.Status != domain.NoteStatusPublished {
				continue
			}
			authors = append(authors, note.AuthorID)
		}
		following, err := s.followingSet(ctx, userID, authors)
		if err != nil {
			return domain.NotePage{}, err
		}
		for _, id := range ids {
			exclusiveMax = id
			note, ok := notes[id]
			if !ok || note.Status != domain.NoteStatusPublished {
				continue
			}
			if _, ok = following[note.AuthorID]; !ok {
				continue
			}
			items = append(items, note)
			if len(items) == need {
				break
			}
		}
		if len(ids) < readBatch {
			break
		}
	}
	page := domain.NotePage{Items: items}
	if len(items) > limit {
		page.HasMore = true
		page.Items = items[:limit]
	}
	if len(page.Items) > 0 {
		page.NextCursor = encodeFeedCursor(page.Items[len(page.Items)-1].ID)
	}
	return page, nil
}

func (s *FeedServiceImpl) followingSet(ctx context.Context, followerID int64, authorIDs []int64) (map[int64]struct{}, error) {
	ids, err := s.follows.ListFolloweeIDs(ctx, followerID, authorIDs)
	if err != nil {
		return nil, err
	}
	set := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	return set, nil
}

func encodeFeedCursor(noteID int64) string {
	return strconv.FormatInt(noteID, 10)
}

func parseFeedCursor(raw string) (int64, error) {
	if raw == "" {
		return 0, nil
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidNoteCursor
	}
	return id, nil
}
