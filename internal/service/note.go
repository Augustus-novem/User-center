package service

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

var (
	ErrInvalidNote       = errors.New("笔记内容不合法")
	ErrNoteNotFound      = repository.ErrNoteNotFound
	ErrNoteDeleted       = repository.ErrNoteDeleted
	ErrNoteForbidden     = errors.New("无权操作该笔记")
	ErrInvalidNoteID     = errors.New("笔记 ID 无效")
	ErrInvalidNoteCursor = errors.New("游标无效")
)

const (
	maxNoteTitleLen   = 80
	maxNoteContentLen = 5000
	maxNoteImages     = 9
	defaultNotePage   = 20
	maxNotePage       = 50
)

type NoteService interface {
	Publish(ctx context.Context, authorID int64, title, content string, imageURLs []string) (domain.Note, error)
	Get(ctx context.Context, noteID int64) (domain.Note, error)
	Delete(ctx context.Context, operatorID, noteID int64) error
	ListByAuthor(ctx context.Context, authorID int64, cursor string, limit int) (domain.NotePage, error)
}

type NoteServiceImpl struct {
	notes     repository.NoteRepository
	tx        repository.Transaction
	publisher events.Publisher
	logger    logger.Logger
}

func NewNoteServiceImpl(notes repository.NoteRepository, tx repository.Transaction, publisher events.Publisher, l logger.Logger) *NoteServiceImpl {
	return &NoteServiceImpl{
		notes:     notes,
		tx:        tx,
		publisher: publisher,
		logger:    l,
	}
}

func (s *NoteServiceImpl) Publish(ctx context.Context, authorID int64, title, content string, imageURLs []string) (domain.Note, error) {
	note, err := buildPublishNote(authorID, title, content, imageURLs)
	if err != nil {
		return domain.Note{}, err
	}
	var created domain.Note
	err = s.tx.InTx(ctx, func(txCtx context.Context) error {
		var txErr error
		created, txErr = s.notes.Create(txCtx, note)
		if txErr != nil {
			return txErr
		}
		return s.publishNotePublished(txCtx, created)
	})
	if err != nil {
		s.logger.Error("publish note failed",
			logger.Field{Key: "user_id", Value: authorID},
			logger.Error(err),
		)
		return domain.Note{}, err
	}
	if invalidator, ok := s.notes.(repository.NoteCacheInvalidator); ok {
		invalidator.Invalidate(ctx, created.ID)
	}
	return created, nil
}

func (s *NoteServiceImpl) Get(ctx context.Context, noteID int64) (domain.Note, error) {
	if noteID <= 0 {
		return domain.Note{}, ErrInvalidNoteID
	}
	note, err := s.notes.FindByID(ctx, noteID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) || errors.Is(err, repository.ErrNoteDeleted) {
			return domain.Note{}, ErrNoteNotFound
		}
		return domain.Note{}, err
	}
	return note, nil
}

func (s *NoteServiceImpl) Delete(ctx context.Context, operatorID, noteID int64) error {
	if operatorID <= 0 || noteID <= 0 {
		return ErrInvalidNoteID
	}
	note, err := s.notes.FindByID(ctx, noteID)
	if err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) || errors.Is(err, repository.ErrNoteDeleted) {
			return ErrNoteNotFound
		}
		return err
	}
	if note.AuthorID != operatorID {
		return ErrNoteForbidden
	}
	if err = s.notes.SoftDelete(ctx, noteID, operatorID); err != nil {
		if errors.Is(err, repository.ErrNoteNotFound) {
			return ErrNoteNotFound
		}
		return err
	}
	return nil
}

func (s *NoteServiceImpl) ListByAuthor(ctx context.Context, authorID int64, cursor string, limit int) (domain.NotePage, error) {
	if authorID <= 0 {
		return domain.NotePage{}, ErrInvalidNoteID
	}
	cur, err := parseNoteCursor(cursor)
	if err != nil {
		return domain.NotePage{}, err
	}
	limit = normalizeNoteLimit(limit)
	rows, err := s.notes.ListByAuthor(ctx, authorID, cur, limit+1)
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

func (s *NoteServiceImpl) publishNotePublished(ctx context.Context, note domain.Note) error {
	if s.publisher == nil || !s.publisher.IsEnabled() {
		return nil
	}
	evt := events.NewNotePublishedEvent(note.ID, note.AuthorID)
	return s.publisher.Publish(ctx, events.TopicNotePublished, events.UserIDKey(note.AuthorID), evt)
}

func buildPublishNote(authorID int64, title, content string, imageURLs []string) (domain.Note, error) {
	if authorID <= 0 {
		return domain.Note{}, ErrInvalidNoteID
	}
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || utf8.RuneCountInString(title) > maxNoteTitleLen {
		return domain.Note{}, ErrInvalidNote
	}
	if utf8.RuneCountInString(content) > maxNoteContentLen {
		return domain.Note{}, ErrInvalidNote
	}
	if len(imageURLs) > maxNoteImages {
		return domain.Note{}, ErrInvalidNote
	}
	images := make([]domain.NoteImage, 0, len(imageURLs))
	for i, raw := range imageURLs {
		url := strings.TrimSpace(raw)
		if url == "" || len(url) > 1024 {
			return domain.Note{}, ErrInvalidNote
		}
		images = append(images, domain.NoteImage{URL: url, SortOrder: i})
	}
	return domain.Note{
		AuthorID: authorID,
		Title:    title,
		Content:  content,
		Status:   domain.NoteStatusPublished,
		Images:   images,
	}, nil
}

func normalizeNoteLimit(limit int) int {
	if limit <= 0 {
		return defaultNotePage
	}
	if limit > maxNotePage {
		return maxNotePage
	}
	return limit
}

func encodeNoteCursor(createdAt, id int64) string {
	return strconv.FormatInt(createdAt, 10) + "_" + strconv.FormatInt(id, 10)
}

func parseNoteCursor(raw string) (*domain.FollowCursor, error) {
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, "_")
	if len(parts) != 2 {
		return nil, ErrInvalidNoteCursor
	}
	createdAt, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || createdAt <= 0 {
		return nil, ErrInvalidNoteCursor
	}
	id, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || id <= 0 {
		return nil, ErrInvalidNoteCursor
	}
	return &domain.FollowCursor{CreatedAt: createdAt, ID: id}, nil
}
