package service

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
	"user-center/internal/domain"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

var ErrInvalidSearchQuery = errors.New("搜索关键词不合法")

const maxSearchQueryLength = 100

type NoteSearchResult struct {
	Items    []NoteSearchItem `json:"items"`
	Degraded bool             `json:"degraded"`
}

type NoteSearchItem struct {
	NoteID    int64  `json:"note_id"`
	AuthorID  int64  `json:"author_id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"created_at"`
	Status    string `json:"status"`
}

type SearchService interface {
	SearchNotes(ctx context.Context, query string, limit int) (NoteSearchResult, error)
}

type SearchServiceImpl struct {
	enabled         bool
	index           repository.NoteSearchIndex
	fallback        repository.NoteSearchFallback
	requestTimeout  time.Duration
	fallbackTimeout time.Duration
	fallbackWindow  time.Duration
	maxLimit        int
	now             func() time.Time
	logger          logger.Logger
}

func NewSearchServiceImpl(enabled bool, index repository.NoteSearchIndex, fallback repository.NoteSearchFallback,
	requestTimeout, fallbackTimeout, fallbackWindow time.Duration, maxLimit int, l logger.Logger,
) *SearchServiceImpl {
	return &SearchServiceImpl{
		enabled: enabled, index: index, fallback: fallback,
		requestTimeout: requestTimeout, fallbackTimeout: fallbackTimeout,
		fallbackWindow: fallbackWindow, maxLimit: maxLimit,
		now: time.Now, logger: l,
	}
}

func (s *SearchServiceImpl) SearchNotes(ctx context.Context, query string, limit int) (NoteSearchResult, error) {
	query = strings.TrimSpace(query)
	if query == "" || utf8.RuneCountInString(query) > maxSearchQueryLength {
		return NoteSearchResult{}, ErrInvalidSearchQuery
	}
	limit = s.normalizeLimit(limit)
	if s.enabled {
		searchCtx, cancel := context.WithTimeout(ctx, s.requestTimeout)
		notes, err := s.index.Search(searchCtx, query, limit)
		cancel()
		if err == nil {
			return buildNoteSearchResult(notes, false), nil
		}
		s.logger.Warn("Elasticsearch 查询失败，进入有界 MySQL 降级",
			logger.Field{Key: "query_length", Value: utf8.RuneCountInString(query)},
			logger.Error(err),
		)
	}

	fallbackCtx, cancel := context.WithTimeout(ctx, s.fallbackTimeout)
	defer cancel()
	notes, err := s.fallback.SearchRecentPublished(fallbackCtx, query, s.now().Add(-s.fallbackWindow).UnixMilli(), limit)
	if err != nil {
		return NoteSearchResult{}, err
	}
	return buildNoteSearchResult(notes, true), nil
}

func buildNoteSearchResult(notes []domain.Note, degraded bool) NoteSearchResult {
	result := NoteSearchResult{Items: make([]NoteSearchItem, 0, len(notes)), Degraded: degraded}
	for _, note := range notes {
		result.Items = append(result.Items, NoteSearchItem{
			NoteID: note.ID, AuthorID: note.AuthorID,
			Title: note.Title, Content: note.Content,
			CreatedAt: note.CreatedAt, Status: note.Status,
		})
	}
	return result
}

func (s *SearchServiceImpl) normalizeLimit(limit int) int {
	if limit <= 0 {
		if s.maxLimit < 10 {
			return s.maxLimit
		}
		return 10
	}
	if limit > s.maxLimit {
		return s.maxLimit
	}
	return limit
}
