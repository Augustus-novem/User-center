package events

import (
	"time"

	"github.com/google/uuid"
)

const ActionCheckIn = "checkin"

type UserActivityEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	UserID     int64  `json:"user_id"`
	Action     string `json:"action"`
	BizID      string `json:"biz_id"`
	Points     int    `json:"points"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewUserCheckInEvent(userID int64, bizID string, points int) UserActivityEvent {
	return UserActivityEvent{
		EventID:    uuid.NewString(),
		Type:       TopicUserActivity,
		UserID:     userID,
		Action:     ActionCheckIn,
		BizID:      bizID,
		Points:     points,
		OccurredAt: time.Now().UnixMilli(),
	}
}
