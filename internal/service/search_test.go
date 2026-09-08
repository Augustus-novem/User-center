package service

import (
	"context"
	"errors"
	"testing"
	"time"
	"user-center/internal/domain"
	"user-center/pkg/logger"
)

type searchQueryIndexStub struct {
	searchFn func(context.Context, string, int) ([]domain.Note, error)
}

func (*searchQueryIndexStub) EnsureIndex(context.Context) error        { return nil }
func (*searchQueryIndexStub) Index(context.Context, domain.Note) error { return nil }
func (*searchQueryIndexStub) Delete(context.Context, int64) error      { return nil }
func (s *searchQueryIndexStub) Search(ctx context.Context, q string, limit int) ([]domain.Note, error) {
	return s.searchFn(ctx, q, limit)
}

type searchFallbackStub struct {
	searchFn func(context.Context, string, int64, int) ([]domain.Note, error)
}

func (s *searchFallbackStub) SearchRecentPublished(ctx context.Context, q string, after int64, limit int) ([]domain.Note, error) {
	return s.searchFn(ctx, q, after, limit)
}

func TestSearchServiceUsesElasticsearch(t *testing.T) {
	t.Parallel()
	fallbackCalled := false
	svc := NewSearchServiceImpl(true, &searchQueryIndexStub{searchFn: func(ctx context.Context, q string, limit int) ([]domain.Note, error) {
		if q != "Go" || limit != 5 {
			t.Fatalf("query=%q limit=%d", q, limit)
		}
		return []domain.Note{{ID: 1}}, nil
	}}, &searchFallbackStub{searchFn: func(context.Context, string, int64, int) ([]domain.Note, error) {
		fallbackCalled = true
		return nil, nil
	}}, time.Second, time.Second, 24*time.Hour, 20, logger.NewNoOpLogger())

	result, err := svc.SearchNotes(context.Background(), " Go ", 5)
	if err != nil || result.Degraded || len(result.Items) != 1 || result.Items[0].NoteID != 1 || fallbackCalled {
		t.Fatalf("result=%+v fallback=%v err=%v", result, fallbackCalled, err)
	}
}

func TestSearchServiceFallbackIsBounded(t *testing.T) {
	t.Parallel()
	now := time.UnixMilli(2_000_000_000_000)
	svc := NewSearchServiceImpl(true, &searchQueryIndexStub{searchFn: func(context.Context, string, int) ([]domain.Note, error) {
		return nil, errors.New("ES down")
	}}, &searchFallbackStub{searchFn: func(ctx context.Context, q string, after int64, limit int) ([]domain.Note, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("fallback context must have a deadline")
		}
		if q != "Go" || after != now.Add(-30*24*time.Hour).UnixMilli() || limit != 20 {
			t.Fatalf("query=%q after=%d limit=%d", q, after, limit)
		}
		return []domain.Note{{ID: 2}}, nil
	}}, 20*time.Millisecond, 30*time.Millisecond, 30*24*time.Hour, 20, logger.NewNoOpLogger())
	svc.now = func() time.Time { return now }

	result, err := svc.SearchNotes(context.Background(), "Go", 1000)
	if err != nil || !result.Degraded || len(result.Items) != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestSearchServiceDisabledUsesFallback(t *testing.T) {
	t.Parallel()
	indexCalled := false
	svc := NewSearchServiceImpl(false, &searchQueryIndexStub{searchFn: func(context.Context, string, int) ([]domain.Note, error) {
		indexCalled = true
		return nil, nil
	}}, &searchFallbackStub{searchFn: func(context.Context, string, int64, int) ([]domain.Note, error) {
		return nil, nil
	}}, time.Second, time.Second, time.Hour, 20, logger.NewNoOpLogger())
	result, err := svc.SearchNotes(context.Background(), "Go", 0)
	if err != nil || !result.Degraded || indexCalled {
		t.Fatalf("result=%+v indexCalled=%v err=%v", result, indexCalled, err)
	}
}

func TestSearchServiceRejectsInvalidQuery(t *testing.T) {
	t.Parallel()
	svc := NewSearchServiceImpl(false, &searchQueryIndexStub{}, &searchFallbackStub{}, time.Second, time.Second, time.Hour, 20, logger.NewNoOpLogger())
	if _, err := svc.SearchNotes(context.Background(), "   ", 10); !errors.Is(err, ErrInvalidSearchQuery) {
		t.Fatalf("want ErrInvalidSearchQuery, got %v", err)
	}
}
