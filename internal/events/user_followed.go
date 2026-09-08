package events

import (
	"time"

	"github.com/google/uuid"
)

type UserFollowedEvent struct {
	EventID    string `json:"event_id"`
	Type       string `json:"type"`
	FollowerID int64  `json:"follower_id"`
	FolloweeID int64  `json:"followee_id"`
	OccurredAt int64  `json:"occurred_at"`
}

func NewUserFollowedEvent(followerID, followeeID int64) UserFollowedEvent {
	return UserFollowedEvent{
		EventID:    uuid.NewString(),
		Type:       TopicUserFollowed,
		FollowerID: followerID,
		FolloweeID: followeeID,
		OccurredAt: time.Now().UnixMilli(),
	}
}
