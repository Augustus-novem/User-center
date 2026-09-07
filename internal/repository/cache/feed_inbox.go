package cache

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"

	"github.com/redis/go-redis/v9"
)

var (
	//go:embed lua/feed_inbox_add.lua
	luaFeedInboxAdd string
)

type RedisFeedInbox struct {
	cmd      redis.Cmdable
	maxItems int
}

func NewRedisFeedInbox(cmd redis.Cmdable, maxItems int) *RedisFeedInbox {
	if maxItems <= 0 {
		maxItems = 500
	}
	return &RedisFeedInbox{cmd: cmd, maxItems: maxItems}
}

func FeedInboxKey(userID int64) string {
	return fmt.Sprintf("feed:inbox:%d", userID)
}

func (r *RedisFeedInbox) AddToUsers(ctx context.Context, userIDs []int64, noteID int64) error {
	if len(userIDs) == 0 {
		return nil
	}
	pipe := r.cmd.Pipeline()
	member := strconv.FormatInt(noteID, 10)
	score := float64(noteID)
	max := strconv.Itoa(r.maxItems)
	for _, userID := range userIDs {
		pipe.Eval(ctx, luaFeedInboxAdd, []string{FeedInboxKey(userID)}, score, member, max)
	}
	_, err := pipe.Exec(ctx)
	return err
}

func (r *RedisFeedInbox) List(ctx context.Context, userID int64, exclusiveMaxNoteID int64, limit int) ([]int64, error) {
	if limit <= 0 {
		return nil, nil
	}
	max := "+inf"
	if exclusiveMaxNoteID > 0 {
		max = "(" + strconv.FormatInt(exclusiveMaxNoteID, 10)
	}
	members, err := r.cmd.ZRevRangeByScore(ctx, FeedInboxKey(userID), &redis.ZRangeBy{
		Max:    max,
		Min:    "-inf",
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0, len(members))
	for _, member := range members {
		id, parseErr := strconv.ParseInt(member, 10, 64)
		if parseErr != nil {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
