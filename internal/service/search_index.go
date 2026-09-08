package service

import (
	"context"
	"errors"
	"fmt"
	"time"
	"user-center/internal/repository"
)

const searchRebuildCleanupTimeout = 5 * time.Second

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

func (s *SearchIndexService) Rebuild(ctx context.Context) (indexed int, err error) {
	if s.rebuild == nil || s.batchSize <= 0 {
		return 0, errors.New("search rebuild source or batch size is invalid")
	}
	physicalIndex, err := s.index.BeginRebuild(ctx)
	if err != nil {
		return 0, err
	}
	committed := false
	defer func() {
		if !committed {
			cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), searchRebuildCleanupTimeout)
			defer cancel()
			if cleanupErr := s.index.AbortRebuild(cleanupCtx, physicalIndex); cleanupErr != nil {
				err = errors.Join(err, fmt.Errorf("delete temporary search index %s: %w", physicalIndex, cleanupErr))
			}
		}
	}()
	var afterID int64
	for {
		notes, err := s.rebuild.ListPublishedAfterID(ctx, afterID, s.batchSize)
		if err != nil {
			return indexed, fmt.Errorf("load search rebuild batch after note %d: %w", afterID, err)
		}
		if len(notes) == 0 {
			break
		}
		for _, note := range notes {
			if err = s.index.IndexInto(ctx, physicalIndex, note); err != nil {
				return indexed, fmt.Errorf("index note %d: %w", note.ID, err)
			}
			indexed++
			afterID = note.ID
		}
		if len(notes) < s.batchSize {
			break
		}
	}
	if err = s.index.CommitRebuild(ctx, physicalIndex); err != nil {
		return indexed, fmt.Errorf("commit search rebuild: %w", err)
	}
	committed = true
	return indexed, nil
}
