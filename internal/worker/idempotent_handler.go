package worker

import (
	"context"
	"fmt"
	"time"
	"user-center/internal/repository/cache"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

// IdempotentHandler 幂等处理器
type IdempotentHandler struct {
	handler         MessageHandler
	idempotentCache cache.IdempotentCache
	keyGenerator    IdempotentKeyGenerator
	ttl             time.Duration
	logger          logger.Logger
}

// IdempotentKeyGenerator 幂等键生成器
type IdempotentKeyGenerator func(msg *sarama.ConsumerMessage) string

func NewIdempotentHandler(
	handler MessageHandler,
	idempotentCache cache.IdempotentCache,
	keyGenerator IdempotentKeyGenerator,
	ttl time.Duration,
	l logger.Logger,
) *IdempotentHandler {
	return &IdempotentHandler{
		handler:         handler,
		idempotentCache: idempotentCache,
		keyGenerator:    keyGenerator,
		ttl:             ttl,
		logger:          l,
	}
}

func (h *IdempotentHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	// 生成幂等键
	idempotentKey := h.keyGenerator(msg)
	
	// 检查是否已处理
	exists, err := h.idempotentCache.CheckIdempotentKey(ctx, idempotentKey)
	if err != nil {
		h.logger.Error("检查幂等键失败",
			logger.Field{Key: "key", Value: idempotentKey},
			logger.Field{Key: "error", Value: err},
		)
		// 幂等检查失败，继续处理（保证可用性）
	} else if exists {
		h.logger.Info("消息已处理，跳过",
			logger.Field{Key: "topic", Value: msg.Topic},
			logger.Field{Key: "partition", Value: msg.Partition},
			logger.Field{Key: "offset", Value: msg.Offset},
			logger.Field{Key: "idempotent_key", Value: idempotentKey},
		)
		return nil
	}
	
	// 设置幂等键（处理前）
	isFirst, err := h.idempotentCache.SetIdempotentKey(ctx, idempotentKey, h.ttl)
	if err != nil {
		h.logger.Error("设置幂等键失败",
			logger.Field{Key: "key", Value: idempotentKey},
			logger.Field{Key: "error", Value: err},
		)
		// 继续处理
	} else if !isFirst {
		// 其他实例正在处理或已处理
		h.logger.Info("消息正在被其他实例处理或已处理",
			logger.Field{Key: "topic", Value: msg.Topic},
			logger.Field{Key: "idempotent_key", Value: idempotentKey},
		)
		return nil
	}
	
	// 执行业务处理
	err = h.handler(ctx, msg)
	if err != nil {
		// 处理失败，删除幂等键以允许重试
		if delErr := h.idempotentCache.DeleteIdempotentKey(ctx, idempotentKey); delErr != nil {
			h.logger.Error("删除幂等键失败",
				logger.Field{Key: "key", Value: idempotentKey},
				logger.Field{Key: "error", Value: delErr},
			)
		}
		return err
	}
	
	h.logger.Debug("消息处理成功，幂等键已保留",
		logger.Field{Key: "topic", Value: msg.Topic},
		logger.Field{Key: "idempotent_key", Value: idempotentKey},
		logger.Field{Key: "ttl", Value: h.ttl.String()},
	)
	
	return nil
}

// DefaultIdempotentKeyGenerator 默认幂等键生成器（基于 topic + partition + offset）
func DefaultIdempotentKeyGenerator(msg *sarama.ConsumerMessage) string {
	return fmt.Sprintf("%s:%d:%d", msg.Topic, msg.Partition, msg.Offset)
}

// BusinessIdempotentKeyGenerator 基于业务 ID 的幂等键生成器
func BusinessIdempotentKeyGenerator(businessIDExtractor func(*sarama.ConsumerMessage) string) IdempotentKeyGenerator {
	return func(msg *sarama.ConsumerMessage) string {
		businessID := businessIDExtractor(msg)
		return fmt.Sprintf("%s:biz:%s", msg.Topic, businessID)
	}
}
