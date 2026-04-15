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

//go:embed lua/update_rank_with_version.lua
var updateRankWithVersionScript string

// RankConsistencyCache 热榜缓存一致性方案
type RankConsistencyCache struct {
	cmd                redis.Cmdable
	updateWithVersionSha string
}

func NewRankConsistencyCache(cmd redis.Cmdable) *RankConsistencyCache {
	return &RankConsistencyCache{
		cmd: cmd,
	}
}

// Init 初始化 Lua 脚本
func (c *RankConsistencyCache) Init(ctx context.Context) error {
	sha, err := c.cmd.ScriptLoad(ctx, updateRankWithVersionScript).Result()
	if err != nil {
		return fmt.Errorf("加载 Lua 脚本失败: %w", err)
	}
	c.updateWithVersionSha = sha
	return nil
}

// IncrSignInScoreWithVersion 带版本号的积分增加（解决并发更新问题）
func (c *RankConsistencyCache) IncrSignInScoreWithVersion(
	ctx context.Context,
	userID int64,
	when time.Time,
	delta int64,
	version int64,
) (bool, error) {
	when = when.In(biztime.Location())
	dayStart := biztime.StartOfDay(when)
	dailyKey := c.dailyKey(when)
	monthlyKey := c.monthlyKey(when.Year(), when.Month())
	versionKey := c.versionKey(userID, when)
	member := strconv.FormatInt(userID, 10)
	
	dailyExpireAt := biztime.NextDayStart(when).Add(7 * 24 * time.Hour)
	monthExpireAt := biztime.NextMonthStart(when).Add(365 * 24 * time.Hour)
	dailyScore := float64(delta)*1e13 + float64(biztime.NextDayStart(dayStart).UnixMilli()-when.UnixMilli())
	
	// 使用 Lua 脚本保证原子性
	result, err := c.cmd.EvalSha(ctx, c.updateWithVersionSha, []string{
		dailyKey,
		monthlyKey,
		versionKey,
	}, member, dailyScore, delta, version, dailyExpireAt.Unix(), monthExpireAt.Unix()).Result()
	
	if err != nil {
		return false, err
	}
	
	// result: 1 表示更新成功，0 表示版本冲突
	return result.(int64) == 1, nil
}

// GetVersion 获取用户当前版本号
func (c *RankConsistencyCache) GetVersion(ctx context.Context, userID int64, when time.Time) (int64, error) {
	versionKey := c.versionKey(userID, when)
	val, err := c.cmd.Get(ctx, versionKey).Result()
	if err == redis.Nil {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(val, 10, 64)
}

// RebuildRankCache 重建热榜缓存（从数据库）
func (c *RankConsistencyCache) RebuildRankCache(
	ctx context.Context,
	when time.Time,
	dailyData map[int64]float64,
	monthlyData map[int64]float64,
) error {
	dailyKey := c.dailyKey(when)
	monthlyKey := c.monthlyKey(when.Year(), when.Month())
	
	pipe := c.cmd.Pipeline()
	
	// 删除旧数据
	pipe.Del(ctx, dailyKey)
	pipe.Del(ctx, monthlyKey)
	
	// 批量写入日榜数据
	if len(dailyData) > 0 {
		dailyMembers := make([]redis.Z, 0, len(dailyData))
		for userID, score := range dailyData {
			dailyMembers = append(dailyMembers, redis.Z{
				Score:  score,
				Member: strconv.FormatInt(userID, 10),
			})
		}
		pipe.ZAdd(ctx, dailyKey, dailyMembers...)
		pipe.ExpireAt(ctx, dailyKey, biztime.NextDayStart(when).Add(7*24*time.Hour))
	}
	
	// 批量写入月榜数据
	if len(monthlyData) > 0 {
		monthlyMembers := make([]redis.Z, 0, len(monthlyData))
		for userID, score := range monthlyData {
			monthlyMembers = append(monthlyMembers, redis.Z{
				Score:  score,
				Member: strconv.FormatInt(userID, 10),
			})
		}
		pipe.ZAdd(ctx, monthlyKey, monthlyMembers...)
		pipe.ExpireAt(ctx, monthlyKey, biztime.NextMonthStart(when).Add(365*24*time.Hour))
	}
	
	_, err := pipe.Exec(ctx)
	return err
}

// InvalidateCache 使缓存失效
func (c *RankConsistencyCache) InvalidateCache(ctx context.Context, when time.Time) error {
	dailyKey := c.dailyKey(when)
	monthlyKey := c.monthlyKey(when.Year(), when.Month())
	return c.cmd.Del(ctx, dailyKey, monthlyKey).Err()
}

func (c *RankConsistencyCache) dailyKey(when time.Time) string {
	day := biztime.StartOfDay(when)
	return fmt.Sprintf("rank:active:daily:%04d%02d%02d", day.Year(), day.Month(), day.Day())
}

func (c *RankConsistencyCache) monthlyKey(year int, month time.Month) string {
	return fmt.Sprintf("rank:active:monthly:%04d%02d", year, month)
}

func (c *RankConsistencyCache) versionKey(userID int64, when time.Time) string {
	day := biztime.StartOfDay(when)
	return fmt.Sprintf("rank:version:%d:%04d%02d%02d", userID, day.Year(), day.Month(), day.Day())
}
