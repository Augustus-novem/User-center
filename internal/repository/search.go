package repository

import (
	"context"
	"user-center/internal/domain"
)

// NoteSearchIndex is the boundary for the derived Elasticsearch note index.
// Elasticsearch implementation details must remain in internal/integration.
type NoteSearchIndex interface {
	EnsureIndex(ctx context.Context) error
	Index(ctx context.Context, note domain.Note) error
	Delete(ctx context.Context, noteID int64) error
	Search(ctx context.Context, query string, limit int) ([]domain.Note, error)
}

type NoteRebuildSource interface {
	ListPublishedAfterID(ctx context.Context, afterID int64, limit int) ([]domain.Note, error)
}

type NoteSearchFallback interface {
	SearchRecentPublished(ctx context.Context, query string, createdAfter int64, limit int) ([]domain.Note, error)
}
