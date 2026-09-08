package domain

const (
	NotificationTypeFollow  = "follow"
	NotificationTypeLike    = "like"
	NotificationTypeComment = "comment"
)

type Notification struct {
	ID         int64
	EventID    string
	ReceiverID int64
	ActorID    int64
	Type       string
	BizID      int64
	IsRead     bool
	CreatedAt  int64
}

type NotificationCursor struct {
	CreatedAt int64
	ID        int64
}

type NotificationPage struct {
	Items      []Notification
	NextCursor string
	HasMore    bool
}
