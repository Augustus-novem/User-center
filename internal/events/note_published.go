package events

import (
	"time"

	"github.com/google/uuid"
)

type NotePublishedEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	NoteID     int64  `json:"note_id"`
	AuthorID   int64  `json:"author_id"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewNotePublishedEvent(noteID, authorID int64) NotePublishedEvent {
	return NotePublishedEvent{
		EventID:    uuid.NewString(),
		Type:       TopicNotePublished,
		NoteID:     noteID,
		AuthorID:   authorID,
		OccurredAt: time.Now().UnixMilli(),
	}
}
