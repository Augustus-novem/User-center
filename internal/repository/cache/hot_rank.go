package cache

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strconv"
	"time"
	"user-center/internal/pkg/biztime"

	"github.com/redis/go-redis/v9"
)

//go:embed lua/record_hot_rank_event.lua
var recordHotRankEventScript string

//go:embed lua/create_hot_rank_snapshot.lua
var createHotRankSnapshotScript string

var ErrHotRankSnapshotExpired = errors.New("hot ranking snapshot expired")

type HotNoteScore struct {
	NoteID int64
	Score  float64
}

type HotRankCache interface {
	Record(ctx context.Context, eventID string, noteID int64, occurredAt time.Time, weight int64) (bool, error)
	SnapshotPage(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]HotNoteScore, error)
}

func (c *RedisHotRankCache) SnapshotPage(ctx context.Context, snapshot time.Time, offset, limit int64, create bool) ([]HotNoteScore, error) {
	snapshot = snapshot.In(biztime.Location()).Truncate(time.Minute)
	if create {
		keys := []string{c.snapshotKey(snapshot), c.snapshotReadyKey(snapshot)}
		bucketCount := int(c.window / time.Minute)
		for i := bucketCount - 1; i >= 0; i-- {
			keys = append(keys, c.bucketKey(snapshot.Add(-time.Duration(i)*time.Minute)))
		}
		if err := c.cmd.Eval(ctx, createHotRankSnapshotScript, keys, c.snapshotTTL.Milliseconds()).Err(); err != nil {
			return nil, err
		}
	} else {
		exists, err := c.cmd.Exists(ctx, c.snapshotReadyKey(snapshot)).Result()
		if err != nil {
			return nil, err
		}
		if exists == 0 {
			return nil, ErrHotRankSnapshotExpired
		}
	}
	zs, err := c.cmd.ZRevRangeWithScores(ctx, c.snapshotKey(snapshot), offset, offset+limit-1).Result()
	if err != nil {
		return nil, err
	}
	items := make([]HotNoteScore, 0, len(zs))
	for _, z := range zs {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		noteID, parseErr := strconv.ParseInt(member, 10, 64)
		if parseErr != nil {
			continue
		}
		items = append(items, HotNoteScore{NoteID: noteID, Score: z.Score})
	}
	return items, nil
}

type RedisHotRankCache struct {
	cmd         redis.Cmdable
	prefix      string
	window      time.Duration
	snapshotTTL time.Duration
	dedupTTL    time.Duration
}

func NewRedisHotRankCache(cmd redis.Cmdable, window, snapshotTTL, dedupTTL time.Duration) *RedisHotRankCache {
	return newRedisHotRankCache(cmd, window, snapshotTTL, dedupTTL, "hot")
}

func newRedisHotRankCache(cmd redis.Cmdable, window, snapshotTTL, dedupTTL time.Duration, prefix string) *RedisHotRankCache {
	return &RedisHotRankCache{
		cmd:         cmd,
		prefix:      prefix,
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
	return fmt.Sprintf("%s:note:%04d%02d%02d%02d%02d", c.prefix,
		minute.Year(), minute.Month(), minute.Day(), minute.Hour(), minute.Minute())
}

func (c *RedisHotRankCache) eventKey(eventID string) string {
	return c.prefix + ":event:done:" + eventID
}

func (c *RedisHotRankCache) snapshotKey(minute time.Time) string {
	return fmt.Sprintf("%s:note:snapshot:%d", c.prefix, minute.Unix()/60)
}

func (c *RedisHotRankCache) snapshotReadyKey(minute time.Time) string {
	return fmt.Sprintf("%s:note:snapshot:ready:%d", c.prefix, minute.Unix()/60)
}
