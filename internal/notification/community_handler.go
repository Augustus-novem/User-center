package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"user-center/internal/events"
	"user-center/internal/service"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type CommunityHandler struct {
	service service.NotificationService
	logger  logger.Logger
}

func NewCommunityHandler(svc service.NotificationService, l logger.Logger) *CommunityHandler {
	return &CommunityHandler{service: svc, logger: l}
}

func (h *CommunityHandler) HandleUserFollowed(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt events.UserFollowedEvent
	if err := decodeCommunityEvent(msg, events.TopicUserFollowed, &evt); err != nil {
		return err
	}
	created, err := h.service.CreateFollow(ctx, evt.EventID, evt.FollowerID, evt.FolloweeID, evt.OccurredAt)
	return h.finish(err, created, evt.EventID, evt.FolloweeID, evt.FollowerID, 0)
}

func (h *CommunityHandler) HandleNoteLiked(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt events.NoteLikedEvent
	if err := decodeCommunityEvent(msg, events.TopicNoteLiked, &evt); err != nil {
		return err
	}
	created, err := h.service.CreateLike(ctx, evt.EventID, evt.UserID, evt.NoteID, evt.OccurredAt)
	return h.finish(err, created, evt.EventID, 0, evt.UserID, evt.NoteID)
}

func (h *CommunityHandler) HandleCommentCreated(ctx context.Context, msg *sarama.ConsumerMessage) error {
	var evt events.CommentCreatedEvent
	if err := decodeCommunityEvent(msg, events.TopicCommentCreated, &evt); err != nil {
		return err
	}
	created, err := h.service.CreateComment(ctx, evt.EventID, evt.UserID, evt.NoteID, evt.CommentID, evt.OccurredAt)
	return h.finish(err, created, evt.EventID, 0, evt.UserID, evt.NoteID)
}

func decodeCommunityEvent(msg *sarama.ConsumerMessage, expectedType string, target any) error {
	if msg == nil {
		return fmt.Errorf("decode %s event: nil message", expectedType)
	}
	if err := json.Unmarshal(msg.Value, target); err != nil {
		return fmt.Errorf("decode %s event: %w", expectedType, err)
	}
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(msg.Value, &envelope); err != nil {
		return fmt.Errorf("decode %s envelope: %w", expectedType, err)
	}
	if envelope.Type != expectedType {
		return fmt.Errorf("decode %s event: unexpected type %q", expectedType, envelope.Type)
	}
	return nil
}

func (h *CommunityHandler) finish(err error, created bool, eventID string, receiverID, actorID, noteID int64) error {
	if err != nil {
		return err
	}
	fields := []logger.Field{
		{Key: "event_id", Value: eventID},
		{Key: "actor_id", Value: actorID},
	}
	if receiverID > 0 {
		fields = append(fields, logger.Field{Key: "receiver_id", Value: receiverID})
	}
	if noteID > 0 {
		fields = append(fields, logger.Field{Key: "note_id", Value: noteID})
	}
	if created {
		h.logger.Info("社区通知写入成功", fields...)
		return nil
	}
	fields = append(fields, logger.Field{Key: "reason", Value: "duplicate_or_self_action"})
	h.logger.Info("社区通知无需重复写入", fields...)
	return nil
}
