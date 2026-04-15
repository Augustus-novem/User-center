package examples

import (
	"context"
	"fmt"
	"time"
	"user-center/internal/repository/cache"
	"user-center/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// 热榜缓存一致性示例
func RankConsistencyExample(redisClient redis.Cmdable, l logger.Logger) {
	ctx := context.Background()
	
	// 1. 初始化一致性缓存
	rankCache := cache.NewRankConsistencyCache(redisClient)
	if err := rankCache.Init(ctx); err != nil {
		l.Error("初始化 Lua 脚本失败", logger.Field{Key: "error", Value: err})
		return
	}

	// 2. 带版本号的更新（解决并发问题）
	userID := int64(12345)
	now := time.Now()
	
	// 获取当前版本
	version, err := rankCache.GetVersion(ctx, userID, now)
	if err != nil {
		l.Error("获取版本失败", logger.Field{Key: "error", Value: err})
		return
	}

	// 尝试更新积分
	success, err := rankCache.IncrSignInScoreWithVersion(ctx, userID, now, 10, version)
	if err != nil {
		l.Error("更新积分失败", logger.Field{Key: "error", Value: err})
		return
	}

	if !success {
		// 版本冲突，需要重试
		l.Warn("版本冲突，准备重试",
			logger.Field{Key: "user_id", Value: userID},
			logger.Field{Key: "version", Value: version},
		)
		
		// 重新获取版本并重试
		newVersion, _ := rankCache.GetVersion(ctx, userID, now)
		success, err = rankCache.IncrSignInScoreWithVersion(ctx, userID, now, 10, newVersion)
		if err != nil || !success {
			l.Error("重试失败", logger.Field{Key: "error", Value: err})
			return
		}
	}

	l.Info("积分更新成功",
		logger.Field{Key: "user_id", Value: userID},
		logger.Field{Key: "delta", Value: 10},
	)
}

// 缓存重建示例
func RebuildRankCacheExample(redisClient redis.Cmdable, l logger.Logger) {
	ctx := context.Background()
	rankCache := cache.NewRankConsistencyCache(redisClient)
	
	// 从数据库查询数据
	dailyData := map[int64]float64{
		1001: 100.0,
		1002: 95.0,
		1003: 90.0,
	}
	
	monthlyData := map[int64]float64{
		1001: 1000.0,
		1002: 950.0,
		1003: 900.0,
	}

	// 重建缓存
	now := time.Now()
	if err := rankCache.RebuildRankCache(ctx, now, dailyData, monthlyData); err != nil {
		l.Error("重建缓存失败", logger.Field{Key: "error", Value: err})
		return
	}

	l.Info("缓存重建成功")
}

// 缓存失效示例
func InvalidateRankCacheExample(redisClient redis.Cmdable, l logger.Logger) {
	ctx := context.Background()
	rankCache := cache.NewRankConsistencyCache(redisClient)
	
	// 数据变更时，主动失效缓存
	now := time.Now()
	if err := rankCache.InvalidateCache(ctx, now); err != nil {
		l.Error("缓存失效失败", logger.Field{Key: "error", Value: err})
		return
	}

	l.Info("缓存已失效")
}

// 并发更新测试
func ConcurrentUpdateExample(redisClient redis.Cmdable, l logger.Logger) {
	ctx := context.Background()
	rankCache := cache.NewRankConsistencyCache(redisClient)
	rankCache.Init(ctx)

	userID := int64(12345)
	now := time.Now()

	// 模拟10个并发更新
	for i := 0; i < 10; i++ {
		go func(index int) {
			for retry := 0; retry < 3; retry++ {
				version, err := rankCache.GetVersion(ctx, userID, now)
				if err != nil {
					l.Error("获取版本失败", logger.Field{Key: "error", Value: err})
					return
				}

				success, err := rankCache.IncrSignInScoreWithVersion(ctx, userID, now, 1, version)
				if err != nil {
					l.Error("更新失败", logger.Field{Key: "error", Value: err})
					return
				}

				if success {
					l.Info("更新成功",
						logger.Field{Key: "goroutine", Value: index},
						logger.Field{Key: "retry", Value: retry},
					)
					return
				}

				// 版本冲突，重试
				l.Warn("版本冲突，重试",
					logger.Field{Key: "goroutine", Value: index},
					logger.Field{Key: "retry", Value: retry},
				)
				time.Sleep(10 * time.Millisecond)
			}

			l.Error("重试次数耗尽", logger.Field{Key: "goroutine", Value: index})
		}(i)
	}

	// 等待所有 goroutine 完成
	time.Sleep(2 * time.Second)
	
	// 验证最终结果
	version, _ := rankCache.GetVersion(ctx, userID, now)
	fmt.Printf("最终版本号: %d (期望: 10)\n", version)
}
