package service

import (
	"context"
	"errors"
	"fmt"
	"user-center/internal/repository"
)

type SearchIndexService struct {
	notes     repository.NoteRepository
	rebuild   repository.NoteRebuildSource
	index     repository.NoteSearchIndex
	batchSize int
}

func NewSearchIndexService(notes repository.NoteRepository, rebuild repository.NoteRebuildSource, index repository.NoteSearchIndex, batchSize int) *SearchIndexService {
	return &SearchIndexService{notes: notes, rebuild: rebuild, index: index, batchSize: batchSize}
}

func (s *SearchIndexService) IndexPublished(ctx context.Context, noteID int64) error {
	note, err := s.notes.FindByID(ctx, noteID)
	if errors.Is(err, repository.ErrNoteDeleted) {
		return s.index.Delete(ctx, noteID)
	}
	if err != nil {
		return err
	}
	return s.index.Index(ctx, note)
}

func (s *SearchIndexService) Delete(ctx context.Context, noteID int64) error {
	return s.index.Delete(ctx, noteID)
}

func (s *SearchIndexService) Rebuild(ctx context.Context) (int, error) {
	if s.rebuild == nil || s.batchSize <= 0 {
		return 0, errors.New("search rebuild source or batch size is invalid")
	}
	if err := s.index.EnsureIndex(ctx); err != nil {
		return 0, err
	}
	indexed := 0
	var afterID int64
	for {
		notes, err := s.rebuild.ListPublishedAfterID(ctx, afterID, s.batchSize)
		if err != nil {
			return indexed, fmt.Errorf("load search rebuild batch after note %d: %w", afterID, err)
		}
		if len(notes) == 0 {
			return indexed, nil
		}
		for _, note := range notes {
			if err = s.index.Index(ctx, note); err != nil {
				return indexed, fmt.Errorf("index note %d: %w", note.ID, err)
			}
			indexed++
			afterID = note.ID
		}
		if len(notes) < s.batchSize {
			return indexed, nil
		}
	}
}
