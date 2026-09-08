package service

import (
	"context"
	"sort"
	"strconv"
	"user-center/internal/domain"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

const (
	celebrityFollowPage = 50
)

type FeedService interface {
	ListFollowing(ctx context.Context, userID int64, cursor string, limit int) (domain.NotePage, error)
	FanoutPublished(ctx context.Context, noteID, authorID int64) error
}

type FeedServiceImpl struct {
	inbox       repository.FeedInbox
	notes       repository.NoteRepository
	follows     repository.FollowRepository
	fanoutBatch int
	threshold   int
	logger      logger.Logger
}

func NewFeedServiceImpl(
	inbox repository.FeedInbox,
	notes repository.NoteRepository,
	follows repository.FollowRepository,
	fanoutBatch int,
	threshold int,
	l logger.Logger,
) *FeedServiceImpl {
	if fanoutBatch <= 0 {
		fanoutBatch = 200
	}
	if threshold < 0 {
		threshold = 0
	}
	if l == nil {
		l = logger.NewNoOpLogger()
	}
	return &FeedServiceImpl{
		inbox:       inbox,
		notes:       notes,
		follows:     follows,
		fanoutBatch: fanoutBatch,
		threshold:   threshold,
		logger:      l,
	}
}

func (s *FeedServiceImpl) FanoutPublished(ctx context.Context, noteID, authorID int64) error {
	if noteID <= 0 || authorID <= 0 {
		return ErrInvalidNoteID
	}
	if s.threshold > 0 {
		n, err := s.follows.CountFollowers(ctx, authorID)
		if err != nil {
			return err
		}
		if n >= int64(s.threshold) {
			s.logger.Info("跳过大 V 扇出",
				logger.Field{Key: "note_id", Value: noteID},
				logger.Field{Key: "user_id", Value: authorID},
				logger.Field{Key: "follower_count", Value: n},
				logger.Field{Key: "fanout_threshold", Value: s.threshold},
			)
			return nil
		}
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
	readBatch := need
	if readBatch < 20 {
		readBatch = 20
	}
	pushCandidates := make([]domain.Note, 0, need)
	seenPush := make(map[int64]struct{}, need)
	inboxMax := exclusiveMax
	for len(pushCandidates) < need {
		ids, err := s.inbox.List(ctx, userID, inboxMax, readBatch)
		if err != nil {
			return domain.NotePage{}, err
		}
		hydrated, err := s.hydrateInbox(ctx, userID, ids)
		if err != nil {
			return domain.NotePage{}, err
		}
		for _, note := range hydrated {
			if _, ok := seenPush[note.ID]; ok {
				continue
			}
			seenPush[note.ID] = struct{}{}
			pushCandidates = append(pushCandidates, note)
			if len(pushCandidates) == need {
				break
			}
		}
		if len(ids) == 0 {
			break
		}
		inboxMax = ids[len(ids)-1]
		if len(ids) < readBatch {
			break
		}
	}
	celebNotes, err := s.celebrityNotes(ctx, userID, exclusiveMax, need)
	if err != nil {
		return domain.NotePage{}, err
	}
	items := mergeNotesByIDDesc(pushCandidates, celebNotes)
	if len(items) > need {
		items = items[:need]
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

func (s *FeedServiceImpl) hydrateInbox(ctx context.Context, userID int64, ids []int64) ([]domain.Note, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	notes, err := s.notes.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	out := make([]domain.Note, 0, len(ids))
	for _, id := range ids {
		note, ok := notes[id]
		if !ok || note.Status != domain.NoteStatusPublished {
			continue
		}
		if _, ok = following[note.AuthorID]; !ok {
			continue
		}
		out = append(out, note)
	}
	return out, nil
}

func (s *FeedServiceImpl) celebrityNotes(ctx context.Context, userID, exclusiveMax int64, limit int) ([]domain.Note, error) {
	if s.threshold <= 0 {
		return nil, nil
	}
	var cursor *domain.FollowCursor
	out := make([]domain.Note, 0)
	for {
		rels, err := s.follows.ListFollowing(ctx, userID, cursor, celebrityFollowPage)
		if err != nil {
			return nil, err
		}
		if len(rels) == 0 {
			break
		}
		ids := make([]int64, 0, len(rels))
		for _, rel := range rels {
			ids = append(ids, rel.FolloweeID)
		}
		celebs, err := s.follows.FilterIDsByMinFollowers(ctx, ids, s.threshold)
		if err != nil {
			return nil, err
		}
		for _, authorID := range celebs {
			rows, err := s.notes.ListPublishedBefore(ctx, authorID, exclusiveMax, limit)
			if err != nil {
				return nil, err
			}
			out = append(out, rows...)
		}
		if len(rels) < celebrityFollowPage {
			break
		}
		last := rels[len(rels)-1]
		cursor = &domain.FollowCursor{CreatedAt: last.CreatedAt, ID: last.ID}
	}
	return out, nil
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

func mergeNotesByIDDesc(a, b []domain.Note) []domain.Note {
	merged := make(map[int64]domain.Note, len(a)+len(b))
	for _, note := range a {
		merged[note.ID] = note
	}
	for _, note := range b {
		merged[note.ID] = note
	}
	ids := make([]int64, 0, len(merged))
	for id := range merged {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] > ids[j] })
	out := make([]domain.Note, 0, len(ids))
	for _, id := range ids {
		out = append(out, merged[id])
	}
	return out
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
