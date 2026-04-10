package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type SignInCache interface {
	SetSigned(ctx context.Context, uid int64, signDate time.Time) error
	IsSignedOnDate(ctx context.Context, userID int64, signDate time.Time) (bool, error)
	GetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month) ([]int, bool, error)
	BatchSetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month, days []int) error
}

type RedisSignInCache struct {
	cmd redis.Cmdable
}

func NewRedisSignInCache(cmd redis.Cmdable) *RedisSignInCache {
	return &RedisSignInCache{cmd: cmd}
}

func (c *RedisSignInCache) SetSigned(ctx context.Context, uid int64, signDate time.Time) error {
	offset := int64(signDate.Day() - 1)
	key := c.key(uid, signDate.Year(), signDate.Month())
	_, err := c.cmd.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.SetBit(ctx, key, offset, 1)
		pipe.Expire(ctx, key, 180*24*time.Hour)
		return nil
	})
	return err
}

func (c *RedisSignInCache) IsSignedOnDate(ctx context.Context, userID int64, signDate time.Time) (bool, error) {
	offset := int64(signDate.Day() - 1)
	key := c.key(userID, signDate.Year(), signDate.Month())
	bit, err := c.cmd.GetBit(ctx, key, offset).Result()
	if err != nil {
		return false, err
	}
	return bit == 1, nil
}

func (c *RedisSignInCache) GetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month) ([]int, bool, error) {
	key := c.key(userID, year, month)
	exists, err := c.cmd.Exists(ctx, key).Result()
	if err != nil {
		return nil, false, err
	}
	if exists == 0 {
		return nil, false, nil
	}
	daysInMonth := daysOfMonth(year, month)
	pipe := c.cmd.Pipeline()
	cmds := make([]*redis.IntCmd, 0, daysInMonth)
	for i := 0; i < daysInMonth; i++ {
		cmds = append(cmds, pipe.GetBit(ctx, key, int64(i)))
	}
	_, err = pipe.Exec(ctx)
	if err != nil {
		return nil, false, err
	}
	res := make([]int, 0, daysInMonth)
	for idx, cmd := range cmds {
		bit, e := cmd.Result()
		if e != nil {
			return nil, false, e
		}
		if bit == 1 {
			res = append(res, idx+1)
		}
	}
	return res, true, nil
}

func (c *RedisSignInCache) BatchSetMonthSignedDays(ctx context.Context, userID int64, year int, month time.Month, days []int) error {
	if len(days) == 0 {
		return nil
	}
	key := c.key(userID, year, month)
	_, err := c.cmd.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for _, day := range days {
			pipe.SetBit(ctx, key, int64(day-1), 1)
		}
		pipe.Expire(ctx, key, 180*24*time.Hour)
		return nil
	})
	return err
}

func (c *RedisSignInCache) key(userID int64, year int, month time.Month) string {
	return fmt.Sprintf("sign:%d:%04d:%02d", userID, year, int(month))
}

func daysOfMonth(year int, month time.Month) int {
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}
