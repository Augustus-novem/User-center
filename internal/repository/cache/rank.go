package cache

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
	"user-center/internal/pkg/biztime"

	"github.com/redis/go-redis/v9"
)

type RankUserScore struct {
	UserID int64
	Score  float64
}

type RankCache interface {
	IncrSignInScore(ctx context.Context, userID int64, when time.Time, delta int64) error
	TopNDaily(ctx context.Context, day time.Time, limit int64) ([]RankUserScore, error)
	TopNMonthly(ctx context.Context, year int, month time.Month, limit int64) ([]RankUserScore, error)
	GetDailyRank(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error)
	GetMonthlyRank(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error)
}

type RedisRankCache struct {
	cmd redis.Cmdable
}

func NewRedisRankCache(cmd redis.Cmdable) *RedisRankCache {
	return &RedisRankCache{cmd: cmd}
}

func (c *RedisRankCache) IncrSignInScore(ctx context.Context, userID int64, when time.Time, delta int64) error {
	when = when.In(biztime.Location())
	dayStart := biztime.StartOfDay(when)
	dailyKey := c.dailyKey(when)
	monthlyKey := c.monthlyKey(when.Year(), when.Month())
	member := strconv.FormatInt(userID, 10)
	dailyExpireAt := biztime.NextDayStart(when).Add(7 * 24 * time.Hour)
	monthExpireAt := biztime.NextMonthStart(when).Add(365 * 24 * time.Hour)
	dailyScore := float64(delta)*1e13 + float64(biztime.NextDayStart(dayStart).UnixMilli()-when.UnixMilli())
	_, err := c.cmd.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.ZAdd(ctx, dailyKey, redis.Z{Score: dailyScore, Member: member})
		pipe.ExpireAt(ctx, dailyKey, dailyExpireAt)
		pipe.ZIncrBy(ctx, monthlyKey, float64(delta), member)
		pipe.ExpireAt(ctx, monthlyKey, monthExpireAt)
		return nil
	})
	return err
}

func (c *RedisRankCache) TopNDaily(ctx context.Context, day time.Time, limit int64) ([]RankUserScore, error) {
	return c.topN(ctx, c.dailyKey(day), limit)
}

func (c *RedisRankCache) TopNMonthly(ctx context.Context, year int, month time.Month, limit int64) ([]RankUserScore, error) {
	return c.topN(ctx, c.monthlyKey(year, month), limit)
}

func (c *RedisRankCache) GetDailyRank(ctx context.Context, userID int64, day time.Time) (int64, float64, bool, error) {
	return c.getRank(ctx, c.dailyKey(day), userID)
}

func (c *RedisRankCache) GetMonthlyRank(ctx context.Context, userID int64, year int, month time.Month) (int64, float64, bool, error) {
	return c.getRank(ctx, c.monthlyKey(year, month), userID)
}

func (c *RedisRankCache) topN(ctx context.Context, key string, limit int64) ([]RankUserScore, error) {
	zs, err := c.cmd.ZRevRangeWithScores(ctx, key, 0, limit-1).Result()
	if err != nil {
		return nil, err
	}
	res := make([]RankUserScore, 0, len(zs))
	for _, z := range zs {
		member, ok := z.Member.(string)
		if !ok {
			continue
		}
		uid, err := strconv.ParseInt(member, 10, 64)
		if err != nil {
			continue
		}
		res = append(res, RankUserScore{UserID: uid, Score: z.Score})
	}
	return res, nil
}

func (c *RedisRankCache) getRank(ctx context.Context, key string, userID int64) (int64, float64, bool, error) {
	member := strconv.FormatInt(userID, 10)
	rank, err := c.cmd.ZRevRank(ctx, key, member).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	score, err := c.cmd.ZScore(ctx, key, member).Result()
	if errors.Is(err, redis.Nil) {
		return 0, 0, false, nil
	}
	if err != nil {
		return 0, 0, false, err
	}
	return rank + 1, score, true, nil
}

func (c *RedisRankCache) dailyKey(when time.Time) string {
	day := biztime.StartOfDay(when)
	return fmt.Sprintf("rank:active:daily:%04d%02d%02d", day.Year(), day.Month(), day.Day())
}

func (c *RedisRankCache) monthlyKey(year int, month time.Month) string {
	return fmt.Sprintf("rank:active:monthly:%04d%02d", year, month)
}
