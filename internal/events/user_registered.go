package events

import (
	"time"

	"github.com/google/uuid"
)

type UserRegisteredEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	UserID     int64  `json:"user_id"`
	Email      string `json:"email"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewUserRegisteredEvent(userID int64, email string) UserRegisteredEvent {
	return UserRegisteredEvent{
		EventID:    uuid.NewString(),
		Type:       TopicUserRegistered,
		UserID:     userID,
		Email:      email,
		OccurredAt: time.Now().UnixMilli(),
	}
}
