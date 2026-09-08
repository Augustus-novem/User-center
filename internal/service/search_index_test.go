package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"user-center/internal/domain"
	"user-center/internal/repository"
)

type searchIndexStub struct {
	indexed     []domain.Note
	deleted     []int64
	ensured     int
	begun       int
	committed   int
	aborted     int
	failIndexAt int64
	abortErr    error
}

func (s *searchIndexStub) EnsureIndex(context.Context) error { s.ensured++; return nil }
func (s *searchIndexStub) Index(_ context.Context, note domain.Note) error {
	s.indexed = append(s.indexed, note)
	return nil
}
func (s *searchIndexStub) Delete(_ context.Context, noteID int64) error {
	s.deleted = append(s.deleted, noteID)
	return nil
}
func (s *searchIndexStub) Search(context.Context, string, int) ([]domain.Note, error) {
	return nil, nil
}
func (s *searchIndexStub) BeginRebuild(context.Context) (string, error) {
	s.begun++
	return "notes-rebuild-test", nil
}
func (s *searchIndexStub) IndexInto(_ context.Context, _ string, note domain.Note) error {
	if note.ID == s.failIndexAt {
		return errors.New("index failed")
	}
	s.indexed = append(s.indexed, note)
	return nil
}
func (s *searchIndexStub) CommitRebuild(context.Context, string) error {
	s.committed++
	return nil
}
func (s *searchIndexStub) AbortRebuild(context.Context, string) error {
	s.aborted++
	return s.abortErr
}

type rebuildSourceStub struct {
	notes []domain.Note
}

func (s *rebuildSourceStub) ListPublishedAfterID(_ context.Context, afterID int64, limit int) ([]domain.Note, error) {
	res := make([]domain.Note, 0, limit)
	for _, note := range s.notes {
		if note.ID > afterID && len(res) < limit {
			res = append(res, note)
		}
	}
	return res, nil
}

func TestSearchIndexServicePublishedAndDeleted(t *testing.T) {
	t.Parallel()
	index := &searchIndexStub{}
	repo := &noteRepoStub{findByIDFn: func(ctx context.Context, id int64) (domain.Note, error) {
		if id == 2 {
			return domain.Note{}, repository.ErrNoteDeleted
		}
		return domain.Note{ID: id, Status: domain.NoteStatusPublished}, nil
	}}
	svc := NewSearchIndexService(repo, nil, index, 0)
	if err := svc.IndexPublished(context.Background(), 1); err != nil {
		t.Fatal(err)
	}
	if err := svc.IndexPublished(context.Background(), 2); err != nil {
		t.Fatal(err)
	}
	if len(index.indexed) != 1 || index.indexed[0].ID != 1 || len(index.deleted) != 1 || index.deleted[0] != 2 {
		t.Fatalf("indexed=%+v deleted=%v", index.indexed, index.deleted)
	}
}

func TestSearchIndexServiceRebuildBatches(t *testing.T) {
	t.Parallel()
	index := &searchIndexStub{}
	source := &rebuildSourceStub{notes: []domain.Note{{ID: 1}, {ID: 2}, {ID: 3}}}
	svc := NewSearchIndexService(&noteRepoStub{}, source, index, 2)
	count, err := svc.Rebuild(context.Background())
	if err != nil || count != 3 || index.begun != 1 || index.committed != 1 || index.aborted != 0 || len(index.indexed) != 3 {
		t.Fatalf("count=%d begun=%d committed=%d aborted=%d indexed=%d err=%v", count, index.begun, index.committed, index.aborted, len(index.indexed), err)
	}
}

func TestSearchIndexServiceRebuildFailureAbortsWithoutSwap(t *testing.T) {
	t.Parallel()
	index := &searchIndexStub{failIndexAt: 2}
	source := &rebuildSourceStub{notes: []domain.Note{{ID: 1}, {ID: 2}, {ID: 3}}}
	svc := NewSearchIndexService(&noteRepoStub{}, source, index, 2)
	count, err := svc.Rebuild(context.Background())
	if err == nil || count != 1 || index.committed != 0 || index.aborted != 1 {
		t.Fatalf("count=%d committed=%d aborted=%d err=%v", count, index.committed, index.aborted, err)
	}
}

func TestSearchIndexServiceRebuildReportsCleanupFailure(t *testing.T) {
	t.Parallel()
	cleanupFailure := errors.New("cleanup failed")
	index := &searchIndexStub{failIndexAt: 2, abortErr: cleanupFailure}
	source := &rebuildSourceStub{notes: []domain.Note{{ID: 1}, {ID: 2}}}
	svc := NewSearchIndexService(&noteRepoStub{}, source, index, 2)

	_, err := svc.Rebuild(context.Background())
	if err == nil || !errors.Is(err, cleanupFailure) || index.aborted != 1 {
		t.Fatalf("expected joined cleanup failure and one abort, aborted=%d err=%v", index.aborted, err)
	}
	if !strings.Contains(err.Error(), "index note 2") {
		t.Fatalf("expected original rebuild failure context, got %v", err)
	}
}

func TestSearchIndexServicePropagatesMissingNote(t *testing.T) {
	t.Parallel()
	want := errors.New("database unavailable")
	svc := NewSearchIndexService(&noteRepoStub{findByIDFn: func(context.Context, int64) (domain.Note, error) {
		return domain.Note{}, want
	}}, nil, &searchIndexStub{}, 0)
	if err := svc.IndexPublished(context.Background(), 1); !errors.Is(err, want) {
		t.Fatalf("want %v, got %v", want, err)
	}
}
