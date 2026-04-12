package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const defaultWelcomeMessageTTL = 90 * 24 * time.Hour

type WelcomeMessage struct {
	UserID     int64  `json:"user_id"`
	Email      string `json:"email"`
	Title      string `json:"title"`
	Content    string `json:"content"`
	CreatedAt  int64  `json:"created_at"`
	OccurredAt int64  `json:"occurred_at"`
}

type WelcomeMessageRepository interface {
	SaveIfAbsent(ctx context.Context, msg WelcomeMessage) (bool, error)
}

type RedisWelcomeMessageRepository struct {
	cmd redis.Cmdable
	ttl time.Duration
}

func NewRedisWelcomeMessageRepository(cmd redis.Cmdable) *RedisWelcomeMessageRepository {
	return &RedisWelcomeMessageRepository{cmd: cmd, ttl: defaultWelcomeMessageTTL}
}

func (r *RedisWelcomeMessageRepository) SaveIfAbsent(ctx context.Context, msg WelcomeMessage) (bool, error) {
	bs, err := json.Marshal(msg)
	if err != nil {
		return false, err
	}
	ok, err := r.cmd.SetNX(ctx, r.key(msg.UserID), bs, r.ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (r *RedisWelcomeMessageRepository) key(userID int64) string {
	return fmt.Sprintf("welcome:message:user:%d", userID)
}
