package events

import (
	"time"

	"github.com/google/uuid"
)

type NoteDeletedEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	NoteID     int64  `json:"note_id"`
	AuthorID   int64  `json:"author_id"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewNoteDeletedEvent(noteID, authorID int64) NoteDeletedEvent {
	return NoteDeletedEvent{
		EventID:    uuid.NewString(),
		Type:       TopicNoteDeleted,
		NoteID:     noteID,
		AuthorID:   authorID,
		OccurredAt: time.Now().UnixMilli(),
	}
}
