package cache

import (
	"context"
	_ "embed"
	"fmt"
	"strconv"
	"time"
	"user-center/internal/pkg/biztime"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/record_hot_rank_event.lua
var recordHotRankEventScript string

type HotRankCache interface {
	Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error)
}

type RedisHotRankCache struct {
	cmd         redis.Cmdable
	window      time.Duration
	snapshotTTL time.Duration
	dedupTTL    time.Duration
}

func NewRedisHotRankCache(cmd redis.Cmdable, window, snapshotTTL, dedupTTL time.Duration) *RedisHotRankCache {
	return &RedisHotRankCache{
		cmd:         cmd,
		window:      window,
		snapshotTTL: snapshotTTL,
		dedupTTL:    dedupTTL,
	}
}

func (c *RedisHotRankCache) Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error) {
	bucket := occurredAt.In(biztime.Location()).Truncate(time.Minute)
	expiresAt := bucket.Add(time.Minute).Add(c.window).Add(c.snapshotTTL)
	result, err := c.cmd.Eval(ctx, recordHotRankEventScript,
		[]string{c.bucketKey(bucket), c.eventKey(eventID)},
		weight,
		strconv.FormatInt(noteID, 10),
		expiresAt.UnixMilli(),
		c.dedupTTL.Milliseconds(),
	).Int()
	if err != nil {
		return false, err
	}
	return result == 1, nil
}

func (c *RedisHotRankCache) bucketKey(minute time.Time) string {
	return fmt.Sprintf("hot:note:%04d%02d%02d%02d%02d",
		minute.Year(), minute.Month(), minute.Day(), minute.Hour(), minute.Minute())
}

func (c *RedisHotRankCache) eventKey(eventID string) string {
	return "hot:event:done:" + eventID
}
