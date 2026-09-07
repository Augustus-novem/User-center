package service

import (
	"context"
	"errors"
	"time"
	"user-center/internal/config"
	"user-center/internal/events"
	"user-center/internal/repository"
)

var ErrInvalidHotRankEvent = errors.New("热榜事件不合法")

type HotRankRecorder interface {
	Record(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error)
}

type HotRankServiceImpl struct {
	repo    repository.HotRankRepository
	weights map[string]int64
}

func NewHotRankServiceImpl(repo repository.HotRankRepository, cfg config.HotRankConfig) *HotRankServiceImpl {
	return &HotRankServiceImpl{
		repo: repo,
		weights: map[string]int64{
			events.TopicNotePublished:  cfg.PublishWeight,
			events.TopicNoteLiked:      cfg.LikeWeight,
			events.TopicCommentCreated: cfg.CommentWeight,
		},
	}
}

func (s *HotRankServiceImpl) Record(ctx context.Context, eventID, eventType string, noteID, occurredAt int64) (bool, error) {
	weight, ok := s.weights[eventType]
	if !ok || eventID == "" || noteID <= 0 || occurredAt <= 0 {
		return false, ErrInvalidHotRankEvent
	}
	return s.repo.Record(ctx, eventID, noteID, time.UnixMilli(occurredAt), weight)
}
