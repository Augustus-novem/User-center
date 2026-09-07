package service

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"user-center/internal/events"
	"user-center/internal/pkg/biztime"
	"user-center/internal/repository"
)

var (
	ErrInvalidHotRankEvent  = errors.New("热榜事件不合法")
	ErrInvalidHotRankCursor = errors.New("热榜游标不合法或已过期")
)

const (
	defaultHotRankPageSize = 10
	maxHotRankPageSize     = 100
)

type HotRankItem struct {
	NoteID int64   `json:"note_id"`
	Score  float64 `json:"score"`
	Rank   int64   `json:"rank"`
}

type HotRankPage struct {
	Items          []HotRankItem `json:"items"`
	NextCursor     string        `json:"next_cursor,omitempty"`
	HasMore        bool          `json:"has_more"`
	SnapshotMinute int64         `json:"snapshot_minute"`
}

type HotRankRecorder interface {
	Record(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error)
}

type HotRankService interface {
	HotRankRecorder
	List(ctx context.Context, cursor string, limit int) (HotRankPage, error)
}

type HotRankServiceImpl struct {
	repo    repository.HotRankRepository
	weights map[string]int64
	now     func() time.Time
}

type HotRankWeights struct {
	Publish int64
	Like    int64
	Comment int64
}

func NewHotRankServiceImpl(repo repository.HotRankRepository, weights HotRankWeights) *HotRankServiceImpl {
	return &HotRankServiceImpl{
		repo: repo,
		weights: map[string]int64{
			events.TopicNotePublished:  weights.Publish,
			events.TopicNoteLiked:      weights.Like,
			events.TopicCommentCreated: weights.Comment,
		},
		now: func() time.Time { return time.UnixMilli(biztime.NowMillis()).In(biztime.Location()) },
	}
}

func (s *HotRankServiceImpl) List(ctx context.Context, cursor string, limit int) (HotRankPage, error) {
	limit = normalizeHotRankLimit(limit)
	snapshot, offset, create, err := s.parseCursor(cursor)
	if err != nil {
		return HotRankPage{}, err
	}
	items, err := s.repo.SnapshotPage(ctx, snapshot, offset, int64(limit+1), create)
	if errors.Is(err, repository.ErrHotRankSnapshotExpired) {
		return HotRankPage{}, ErrInvalidHotRankCursor
	}
	if err != nil {
		return HotRankPage{}, err
	}
	page := HotRankPage{SnapshotMinute: snapshot.Unix() / 60}
	if len(items) > limit {
		page.HasMore = true
		items = items[:limit]
	}
	page.Items = make([]HotRankItem, 0, len(items))
	for i, item := range items {
		page.Items = append(page.Items, HotRankItem{NoteID: item.NoteID, Score: item.Score, Rank: offset + int64(i) + 1})
	}
	if page.HasMore {
		page.NextCursor = fmt.Sprintf("%d_%d", page.SnapshotMinute, offset+int64(len(items)))
	}
	return page, nil
}

func (s *HotRankServiceImpl) parseCursor(raw string) (time.Time, int64, bool, error) {
	now := s.now().Truncate(time.Minute)
	if raw == "" {
		return now, 0, true, nil
	}
	parts := strings.Split(raw, "_")
	if len(parts) != 2 {
		return time.Time{}, 0, false, ErrInvalidHotRankCursor
	}
	minute, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || minute <= 0 {
		return time.Time{}, 0, false, ErrInvalidHotRankCursor
	}
	offset, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || offset < 0 {
		return time.Time{}, 0, false, ErrInvalidHotRankCursor
	}
	snapshot := time.Unix(minute*60, 0).In(biztime.Location())
	if snapshot.After(now) {
		return time.Time{}, 0, false, ErrInvalidHotRankCursor
	}
	return snapshot, offset, false, nil
}

func normalizeHotRankLimit(limit int) int {
	if limit <= 0 {
		return defaultHotRankPageSize
	}
	if limit > maxHotRankPageSize {
		return maxHotRankPageSize
	}
	return limit
}

func (s *HotRankServiceImpl) Record(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error) {
	weight, ok := s.weights[eventType]
	if !ok || eventID == "" || noteID <= 0 || occurredAt <= 0 {
		return false, ErrInvalidHotRankEvent
	}
	return s.repo.Record(ctx, eventID, noteID, time.UnixMilli(occurredAt), weight)
}
