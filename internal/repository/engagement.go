package repository

import (
	"context"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var ErrLikeDuplicate = dao.ErrLikeDuplicate

type LikeRepository interface {
	Create(ctx context.Context, userID, noteID int64) error
	Delete(ctx context.Context, userID, noteID int64) error
}

type CommentRepository interface {
	Create(ctx context.Context, comment domain.Comment) (domain.Comment, error)
	ListByNote(ctx context.Context, noteID int64, cursor *domain.FollowCursor, limit int) ([]domain.Comment, error)
}

type LikeRepositoryImpl struct{ dao dao.LikeDAO }

func NewLikeRepositoryImpl(d dao.LikeDAO) *LikeRepositoryImpl {
	return &LikeRepositoryImpl{dao: d}
}

func (r *LikeRepositoryImpl) Create(ctx context.Context, userID, noteID int64) error {
	return r.dao.Insert(ctx, dao.NoteLikeOfDB{UserId: userID, NoteId: noteID})
}

func (r *LikeRepositoryImpl) Delete(ctx context.Context, userID, noteID int64) error {
	return r.dao.Delete(ctx, userID, noteID)
}

type CommentRepositoryImpl struct{ dao dao.CommentDAO }

func NewCommentRepositoryImpl(d dao.CommentDAO) *CommentRepositoryImpl {
	return &CommentRepositoryImpl{dao: d}
}

func (r *CommentRepositoryImpl) Create(ctx context.Context, comment domain.Comment) (domain.Comment, error) {
	row, err := r.dao.Insert(ctx, dao.CommentOfDB{
		NoteId:  comment.NoteID,
		UserId:  comment.UserID,
		Content: comment.Content,
		Status:  domain.CommentStatusPublished,
	})
	if err != nil {
		return domain.Comment{}, err
	}
	return toDomainComment(row), nil
}

func (r *CommentRepositoryImpl) ListByNote(ctx context.Context, noteID int64, cursor *domain.FollowCursor, limit int) ([]domain.Comment, error) {
	var daoCursor *dao.CommentCursor
	if cursor != nil {
		daoCursor = &dao.CommentCursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID}
	}
	rows, err := r.dao.ListByNote(ctx, noteID, domain.CommentStatusPublished, daoCursor, limit)
	if err != nil {
		return nil, err
	}
	res := make([]domain.Comment, 0, len(rows))
	for _, row := range rows {
		res = append(res, toDomainComment(row))
	}
	return res, nil
}

func toDomainComment(row dao.CommentOfDB) domain.Comment {
	return domain.Comment{
		ID:        row.Id,
		NoteID:    row.NoteId,
		UserID:    row.UserId,
		Content:   row.Content,
		Status:    row.Status,
		CreatedAt: row.CreatedAt,
	}
}
