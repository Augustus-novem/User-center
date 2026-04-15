package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// 简化版的热榜一致性缓存（用于测试）
type TestRankCache struct {
	cmd redis.Cmdable
}

func NewTestRankCache(cmd redis.Cmdable) *TestRankCache {
	return &TestRankCache{cmd: cmd}
}

func (c *TestRankCache) GetVersion(ctx context.Context, userID int64) (int64, error) {
	key := fmt.Sprintf("test:rank:version:%d", userID)
	val, err := c.cmd.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 0, nil
	}
	return val, err
}

func (c *TestRankCache) IncrVersion(ctx context.Context, userID int64) error {
	key := fmt.Sprintf("test:rank:version:%d", userID)
	return c.cmd.Incr(ctx, key).Err()
}

func main() {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// 测试连接
	ctx := context.Background()
	if err := client.Ping(ctx).Err(); err != nil {
		fmt.Printf("❌ Redis 连接失败: %v\n", err)
		fmt.Println("请确保 Redis 已启动: redis-server")
		return
	}

	cache := NewTestRankCache(client)
	userID := int64(12345)

	// 清理旧数据
	key := fmt.Sprintf("test:rank:version:%d", userID)
	client.Del(ctx, key)

	fmt.Println("========================================")
	fmt.Println("热榜缓存并发更新测试")
	fmt.Println("========================================")
	fmt.Printf("用户 ID: %d\n", userID)
	fmt.Println("并发数: 20")
	fmt.Println("预期最终版本号: 20")
	fmt.Println()

	var wg sync.WaitGroup
	successCount := 0
	var mu sync.Mutex

	startTime := time.Now()

	// 20 个 goroutine 并发更新
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			// 模拟版本号检查和更新
			for retry := 0; retry < 10; retry++ {
				version, err := cache.GetVersion(ctx, userID)
				if err != nil {
					fmt.Printf("[G%02d] ❌ 获取版本失败: %v\n", index, err)
					time.Sleep(10 * time.Millisecond)
					continue
				}

				// 模拟 CAS 操作
				err = cache.IncrVersion(ctx, userID)
				if err != nil {
					fmt.Printf("[G%02d] ❌ 更新失败: %v\n", index, err)
					time.Sleep(10 * time.Millisecond)
					continue
				}

				// 验证版本号是否正确递增
				newVersion, _ := cache.GetVersion(ctx, userID)
				if newVersion > version {
					mu.Lock()
					successCount++
					mu.Unlock()
					if retry > 0 {
						fmt.Printf("[G%02d] ✅ 更新成功 (重试 %d 次, v%d -> v%d)\n", index, retry, version, newVersion)
					} else {
						fmt.Printf("[G%02d] ✅ 更新成功 (v%d -> v%d)\n", index, version, newVersion)
					}
					return
				}

				// 版本冲突，重试
				fmt.Printf("[G%02d] ⚠️  版本冲突，重试 %d\n", index, retry+1)
				time.Sleep(10 * time.Millisecond)
			}

			fmt.Printf("[G%02d] ❌ 重试次数耗尽\n", index)
		}(i)
	}

	wg.Wait()
	duration := time.Since(startTime)

	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("测试结果")
	fmt.Println("========================================")
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("成功: %d / 20\n", successCount)

	finalVersion, _ := cache.GetVersion(ctx, userID)
	fmt.Printf("最终版本号: %d (预期: 20)\n", finalVersion)
	fmt.Println()

	if finalVersion == 20 && successCount == 20 {
		fmt.Println("✅ 测试通过！所有并发更新都成功了")
	} else {
		fmt.Println("❌ 测试失败！")
		if finalVersion != 20 {
			fmt.Printf("   版本号不正确: 期望 20, 实际 %d\n", finalVersion)
		}
		if successCount != 20 {
			fmt.Printf("   成功数不正确: 期望 20, 实际 %d\n", successCount)
		}
	}

	// 清理测试数据
	client.Del(ctx, key)
}
