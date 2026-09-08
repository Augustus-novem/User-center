package repository

import (
	"context"
	"errors"
	"user-center/internal/domain"
	"user-center/internal/repository/dao"
)

var (
	ErrNoteNotFound = dao.ErrNoteNotFound
	ErrNoteDeleted  = errors.New("笔记已删除")
)

type NoteRepository interface {
	Create(ctx context.Context, note domain.Note) (domain.Note, error)
	FindByID(ctx context.Context, id int64) (domain.Note, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]domain.Note, error)
	ListByAuthor(ctx context.Context, authorID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error)
	ListPublishedBefore(ctx context.Context, authorID, exclusiveMaxID int64, limit int) ([]domain.Note, error)
	SoftDelete(ctx context.Context, id, authorID int64) error
}

// NoteCacheInvalidator is implemented by cache-decorated note repositories.
// It lets the service invalidate a newly allocated ID only after its transaction commits.
type NoteCacheInvalidator interface {
	Invalidate(ctx context.Context, id int64)
}

type NoteRepositoryImpl struct {
	dao dao.NoteDAO
}

func NewNoteRepositoryImpl(d dao.NoteDAO) *NoteRepositoryImpl {
	return &NoteRepositoryImpl{dao: d}
}

func (r *NoteRepositoryImpl) Create(ctx context.Context, note domain.Note) (domain.Note, error) {
	row, err := r.dao.Insert(ctx, dao.NoteOfDB{
		AuthorId: note.AuthorID,
		Title:    note.Title,
		Content:  note.Content,
		Status:   domain.NoteStatusPublished,
	})
	if err != nil {
		return domain.Note{}, err
	}
	images := make([]dao.NoteImageOfDB, 0, len(note.Images))
	for _, img := range note.Images {
		images = append(images, dao.NoteImageOfDB{
			NoteId:    row.Id,
			URL:       img.URL,
			SortOrder: img.SortOrder,
		})
	}
	if err = r.dao.InsertImages(ctx, images); err != nil {
		return domain.Note{}, err
	}
	created := toDomainNote(row, images)
	return created, nil
}

func (r *NoteRepositoryImpl) FindByID(ctx context.Context, id int64) (domain.Note, error) {
	row, err := r.dao.FindByID(ctx, id)
	if err != nil {
		return domain.Note{}, err
	}
	images, err := r.dao.ListImages(ctx, id)
	if err != nil {
		return domain.Note{}, err
	}
	note := toDomainNote(row, images)
	if note.Status == domain.NoteStatusDeleted {
		return domain.Note{}, ErrNoteDeleted
	}
	return note, nil
}

func (r *NoteRepositoryImpl) FindByIDs(ctx context.Context, ids []int64) (map[int64]domain.Note, error) {
	rows, err := r.dao.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	res := make(map[int64]domain.Note, len(rows))
	for _, row := range rows {
		res[row.Id] = toDomainNote(row, nil)
	}
	return res, nil
}

func (r *NoteRepositoryImpl) ListByAuthor(ctx context.Context, authorID int64, cursor *domain.FollowCursor, limit int) ([]domain.Note, error) {
	var daoCursor *dao.NoteCursor
	if cursor != nil {
		daoCursor = &dao.NoteCursor{CreatedAt: cursor.CreatedAt, ID: cursor.ID}
	}
	rows, err := r.dao.ListByAuthor(ctx, authorID, domain.NoteStatusPublished, daoCursor, limit)
	if err != nil {
		return nil, err
	}
	res := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		res = append(res, toDomainNote(row, nil))
	}
	return res, nil
}

func (r *NoteRepositoryImpl) ListPublishedBefore(ctx context.Context, authorID, exclusiveMaxID int64, limit int) ([]domain.Note, error) {
	rows, err := r.dao.ListPublishedBefore(ctx, authorID, exclusiveMaxID, limit)
	if err != nil {
		return nil, err
	}
	res := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		res = append(res, toDomainNote(row, nil))
	}
	return res, nil
}

func (r *NoteRepositoryImpl) ListPublishedAfterID(ctx context.Context, afterID int64, limit int) ([]domain.Note, error) {
	rows, err := r.dao.ListPublishedAfterID(ctx, afterID, limit)
	if err != nil {
		return nil, err
	}
	res := make([]domain.Note, 0, len(rows))
	for _, row := range rows {
		res = append(res, toDomainNote(row, nil))
	}
	return res, nil
}

func (r *NoteRepositoryImpl) SoftDelete(ctx context.Context, id, authorID int64) error {
	return r.dao.SoftDelete(ctx, id, authorID)
}

func toDomainNote(row dao.NoteOfDB, images []dao.NoteImageOfDB) domain.Note {
	note := domain.Note{
		ID:        row.Id,
		AuthorID:  row.AuthorId,
		Title:     row.Title,
		Content:   row.Content,
		Status:    row.Status,
		Images:    make([]domain.NoteImage, 0, len(images)),
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
	for _, img := range images {
		note.Images = append(note.Images, domain.NoteImage{
			URL:       img.URL,
			SortOrder: img.SortOrder,
		})
	}
	return note
}
