package examples

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"user-center/internal/repository/cache"
	"user-center/internal/worker"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

// 幂等处理示例
func IdempotentExample(idempotentCache cache.IdempotentCache, l logger.Logger) {
	// 1. 基于 offset 的幂等（默认）
	businessHandler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		l.Info("处理消息", logger.Field{Key: "offset", Value: msg.Offset})
		return nil
	}

	offsetIdempotentHandler := worker.NewIdempotentHandler(
		businessHandler,
		idempotentCache,
		worker.DefaultIdempotentKeyGenerator, // topic:partition:offset
		24*time.Hour,
		l,
	)

	// 2. 基于业务 ID 的幂等
	type UserEvent struct {
		UserID int64  `json:"user_id"`
		Action string `json:"action"`
	}

	businessIDExtractor := func(msg *sarama.ConsumerMessage) string {
		var event UserEvent
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return ""
		}
		// 使用 user_id + action 作为业务唯一标识
		return fmt.Sprintf("%d:%s", event.UserID, event.Action)
	}

	businessIdempotentHandler := worker.NewIdempotentHandler(
		businessHandler,
		idempotentCache,
		worker.BusinessIdempotentKeyGenerator(businessIDExtractor),
		24*time.Hour,
		l,
	)

	// 3. 组合使用：幂等 + 重试
	// 先幂等检查，再重试处理
	// retryHandler := worker.NewRetryableHandler(businessHandler, producer, l)
	// combinedHandler := worker.NewIdempotentHandler(
	// 	retryHandler.Handle,
	// 	idempotentCache,
	// 	worker.DefaultIdempotentKeyGenerator,
	// 	24*time.Hour,
	// 	l,
	// )

	_ = offsetIdempotentHandler
	_ = businessIdempotentHandler
}

// 手动幂等检查示例
func ManualIdempotentCheckExample(ctx context.Context, idempotentCache cache.IdempotentCache) error {
	businessKey := "order:12345"
	
	// 1. 检查是否已处理
	exists, err := idempotentCache.CheckIdempotentKey(ctx, businessKey)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("订单已处理")
	}

	// 2. 设置幂等键
	isFirst, err := idempotentCache.SetIdempotentKey(ctx, businessKey, 24*time.Hour)
	if err != nil {
		return err
	}
	if !isFirst {
		return fmt.Errorf("订单正在被处理")
	}

	// 3. 执行业务逻辑
	// ... 处理订单 ...

	// 4. 如果失败，删除幂等键允许重试
	// if err := processOrder(); err != nil {
	// 	idempotentCache.DeleteIdempotentKey(ctx, businessKey)
	// 	return err
	// }

	// 5. 成功后保留幂等键
	return nil
}

// 幂等键延期示例
func ExtendIdempotentKeyExample(ctx context.Context, idempotentCache cache.IdempotentCache) error {
	businessKey := "long-running-task:123"
	
	// 长时间运行的任务，需要延长幂等键有效期
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 每10分钟延长一次
			if err := idempotentCache.ExtendIdempotentKey(ctx, businessKey, 30*time.Minute); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
