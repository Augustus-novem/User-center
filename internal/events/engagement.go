package events

import (
	"time"

	"github.com/google/uuid"
)

type NoteLikedEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	NoteID     int64  `json:"note_id"`
	UserID     int64  `json:"user_id"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewNoteLikedEvent(noteID, userID int64) NoteLikedEvent {
	return NoteLikedEvent{
		EventID:    uuid.NewString(),
		Type:       TopicNoteLiked,
		NoteID:     noteID,
		UserID:     userID,
		OccurredAt: time.Now().UnixMilli(),
	}
}

type CommentCreatedEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	CommentID  int64  `json:"comment_id"`
	NoteID     int64  `json:"note_id"`
	UserID     int64  `json:"user_id"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewCommentCreatedEvent(commentID, noteID, userID int64) CommentCreatedEvent {
	return CommentCreatedEvent{
		EventID:    uuid.NewString(),
		Type:       TopicCommentCreated,
		CommentID:  commentID,
		NoteID:     noteID,
		UserID:     userID,
		OccurredAt: time.Now().UnixMilli(),
	}
}
