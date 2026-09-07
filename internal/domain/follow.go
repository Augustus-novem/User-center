package domain

type UserRelation struct {
	ID         int64
	FollowerID int64
	FolloweeID int64
	CreatedAt  int64
}

type FollowListItem struct {
	UserID    int64
	CreatedAt int64
}

type FollowPage struct {
	Items      []FollowListItem
	NextCursor string
	HasMore    bool
}

type FollowCursor struct {
	CreatedAt int64
	ID        int64
}
