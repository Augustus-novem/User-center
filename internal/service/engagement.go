package service

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/pkg/logger"
)

var (
	ErrInvalidComment = errors.New("评论内容不合法")
)

const maxCommentLen = 500

type EngagementService interface {
	Like(ctx context.Context, userID, noteID int64) error
	Unlike(ctx context.Context, userID, noteID int64) error
	CreateComment(ctx context.Context, userID, noteID int64, content string) (domain.Comment, error)
	ListComments(ctx context.Context, noteID int64, cursor string, limit int) (domain.CommentPage, error)
}

type EngagementServiceImpl struct {
	notes     repository.NoteRepository
	likes     repository.LikeRepository
	comments  repository.CommentRepository
	tx        repository.Transaction
	publisher events.Publisher
	logger    logger.Logger
}

func NewEngagementServiceImpl(
	notes repository.NoteRepository,
	likes repository.LikeRepository,
	comments repository.CommentRepository,
	tx repository.Transaction,
	publisher events.Publisher,
	l logger.Logger,
) *EngagementServiceImpl {
	return &EngagementServiceImpl{
		notes:     notes,
		likes:     likes,
		comments:  comments,
		tx:        tx,
		publisher: publisher,
		logger:    l,
	}
}

func (s *EngagementServiceImpl) Like(ctx context.Context, userID, noteID int64) error {
	if userID <= 0 || noteID <= 0 {
		return ErrInvalidNoteID
	}
	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureNotePublished(txCtx, noteID); err != nil {
			return err
		}
		if err := s.likes.Create(txCtx, userID, noteID); err != nil {
			return err
		}
		return s.publishLiked(txCtx, noteID, userID)
	})
	if errors.Is(err, repository.ErrLikeDuplicate) {
		return nil
	}
	return err
}

func (s *EngagementServiceImpl) Unlike(ctx context.Context, userID, noteID int64) error {
	if userID <= 0 || noteID <= 0 {
		return ErrInvalidNoteID
	}
	return s.likes.Delete(ctx, userID, noteID)
}

func (s *EngagementServiceImpl) CreateComment(ctx context.Context, userID, noteID int64, content string) (domain.Comment, error) {
	content = strings.TrimSpace(content)
	if content == "" || utf8.RuneCountInString(content) > maxCommentLen {
		return domain.Comment{}, ErrInvalidComment
	}
	if userID <= 0 || noteID <= 0 {
		return domain.Comment{}, ErrInvalidNoteID
	}
	var created domain.Comment
	err := s.tx.InTx(ctx, func(txCtx context.Context) error {
		if err := s.ensureNotePublished(txCtx, noteID); err != nil {
			return err
		}
		var txErr error
		created, txErr = s.comments.Create(txCtx, domain.Comment{
			NoteID:  noteID,
			UserID:  userID,
			Content: content,
			Status:  domain.CommentStatusPublished,
		})
		if txErr != nil {
			return txErr
		}
		return s.publishCommentCreated(txCtx, created)
	})
	if err != nil {
		return domain.Comment{}, err
	}
	return created, nil
}

func (s *EngagementServiceImpl) ListComments(ctx context.Context, noteID int64, cursor string, limit int) (domain.CommentPage, error) {
	if noteID <= 0 {
		return domain.CommentPage{}, ErrInvalidNoteID
	}
	if err := s.ensureNotePublished(ctx, noteID); err != nil {
		return domain.CommentPage{}, err
	}
	cur, err := parseNoteCursor(cursor)
	if err != nil {
		if errors.Is(err, ErrInvalidNoteCursor) {
			return domain.CommentPage{}, ErrInvalidNoteCursor
		}
		return domain.CommentPage{}, err
	}
	limit = normalizeNoteLimit(limit)
	rows, err := s.comments.ListByNote(ctx, noteID, cur, limit+1)
	if err != nil {
		return domain.CommentPage{}, err
	}
	page := domain.CommentPage{Items: rows}
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

func (s *EngagementServiceImpl) ensureNotePublished(ctx context.Context, noteID int64) error {
	_, err := s.notes.FindByID(ctx, noteID)
	if errors.Is(err, repository.ErrNoteNotFound) || errors.Is(err, repository.ErrNoteDeleted) {
		return ErrNoteNotFound
	}
	return err
}

func (s *EngagementServiceImpl) publishLiked(ctx context.Context, noteID, userID int64) error {
	if s.publisher == nil || !s.publisher.IsEnabled() {
		return nil
	}
	evt := events.NewNoteLikedEvent(noteID, userID)
	return s.publisher.Publish(ctx, events.TopicNoteLiked, events.UserIDKey(userID), evt)
}

func (s *EngagementServiceImpl) publishCommentCreated(ctx context.Context, comment domain.Comment) error {
	if s.publisher == nil || !s.publisher.IsEnabled() {
		return nil
	}
	evt := events.NewCommentCreatedEvent(comment.ID, comment.NoteID, comment.UserID)
	return s.publisher.Publish(ctx, events.TopicCommentCreated, events.UserIDKey(comment.UserID), evt)
}
