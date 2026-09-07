package repository

import "context"

type FeedInbox interface {
	AddToUsers(ctx context.Context, userIDs []int64, noteID int64) error
	List(ctx context.Context, userID int64, exclusiveMaxNoteID int64, limit int) ([]int64, error)
}
