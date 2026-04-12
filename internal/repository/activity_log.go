package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type ActivityLogEntry struct {
	UserID     int64  `json:"user_id"`
	Action     string `json:"action"`
	BizID      string `json:"biz_id"`
	Points     int    `json:"points"`
	OccurredAt int64  `json:"occurred_at"`
}

type ActivityLogRepository interface {
	Append(ctx context.Context, entry ActivityLogEntry) error
}

type RedisActivityLogRepository struct {
	cmd redis.Cmdable
}

func NewRedisActivityLogRepository(cmd redis.Cmdable) *RedisActivityLogRepository {
	return &RedisActivityLogRepository{cmd: cmd}
}

func (r *RedisActivityLogRepository) Append(ctx context.Context, entry ActivityLogEntry) error {
	bs, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	key := fmt.Sprintf("activity:log:user:%d", entry.UserID)
	_, err = r.cmd.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.LPush(ctx, key, bs)
		pipe.LTrim(ctx, key, 0, 99)
		pipe.Expire(ctx, key, 30*24*time.Hour)
		return nil
	})
	return err
}
