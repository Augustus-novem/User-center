package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
	"user-center/internal/events"
	"user-center/internal/pkg/biztime"

	"github.com/redis/go-redis/v9"
)

type UserActivityProcessor interface {
	ProcessOnce(ctx context.Context, evt events.UserActivityEvent) (bool, error)
}

type RedisUserActivityProcessor struct {
	cmd      redis.Cmdable
	doneTTL  time.Duration
	logTTL   time.Duration
	logLimit int64
}

func NewRedisUserActivityProcessor(cmd redis.Cmdable) *RedisUserActivityProcessor {
	return &RedisUserActivityProcessor{
		cmd:      cmd,
		doneTTL:  7 * 24 * time.Hour,
		logTTL:   30 * 24 * time.Hour,
		logLimit: 100,
	}
}

func (p *RedisUserActivityProcessor) ProcessOnce(ctx context.Context, evt events.UserActivityEvent) (bool, error) {
	entry, err := json.Marshal(struct {
		UserID     int64  `json:"user_id"`
		Action     string `json:"action"`
		BizID      string `json:"biz_id"`
		Points     int    `json:"points"`
		OccurredAt int64  `json:"occurred_at"`
	}{
		UserID:     evt.UserID,
		Action:     evt.Action,
		BizID:      evt.BizID,
		Points:     evt.Points,
		OccurredAt: evt.OccurredAt,
	})
	if err != nil {
		return false, err
	}

	doneKey := fmt.Sprintf("worker:event:done:%s", evt.EventID)
	activityKey := fmt.Sprintf("activity:log:user:%d", evt.UserID)
	keys := []string{doneKey, activityKey, "", ""}
	args := []any{
		p.doneTTL.Milliseconds(),
		string(entry),
		p.logLimit - 1,
		p.logTTL.Milliseconds(),
		"",
		"",
		0,
		0,
		0,
		0,
	}

	if evt.Action == events.ActionCheckIn && evt.Points > 0 {
		when := time.UnixMilli(evt.OccurredAt).In(biztime.Location())
		dayStart := biztime.StartOfDay(when)
		dailyExpireAt := biztime.NextDayStart(when).Add(7 * 24 * time.Hour).Unix()
		monthExpireAt := biztime.NextMonthStart(when).Add(365 * 24 * time.Hour).Unix()
		dailyScore := float64(evt.Points)*1e13 + float64(biztime.NextDayStart(dayStart).UnixMilli()-when.UnixMilli())
		keys[2] = fmt.Sprintf("rank:active:daily:%04d%02d%02d", dayStart.Year(), dayStart.Month(), dayStart.Day())
		keys[3] = fmt.Sprintf("rank:active:monthly:%04d%02d", when.Year(), when.Month())
		args[4] = strconv.FormatFloat(dailyScore, 'f', -1, 64)
		args[5] = strconv.FormatInt(evt.UserID, 10)
		args[6] = dailyExpireAt
		args[7] = evt.Points
		args[8] = monthExpireAt
		args[9] = evt.UserID
	}

	res, err := p.cmd.Eval(ctx, redisProcessUserActivityScript, keys, args...).Int()
	if err != nil {
		return false, err
	}
	return res == 1, nil
}

const redisProcessUserActivityScript = `
if redis.call('EXISTS', KEYS[1]) == 1 then
	return 0
end
redis.call('LPUSH', KEYS[2], ARGV[2])
redis.call('LTRIM', KEYS[2], 0, tonumber(ARGV[3]))
redis.call('PEXPIRE', KEYS[2], tonumber(ARGV[4]))
if KEYS[3] ~= '' and KEYS[4] ~= '' then
	redis.call('ZADD', KEYS[3], tonumber(ARGV[5]), ARGV[6])
	redis.call('EXPIREAT', KEYS[3], tonumber(ARGV[7]))
	redis.call('ZINCRBY', KEYS[4], tonumber(ARGV[8]), ARGV[10])
	redis.call('EXPIREAT', KEYS[4], tonumber(ARGV[9]))
end
redis.call('SET', KEYS[1], '1', 'PX', tonumber(ARGV[1]))
return 1
`
