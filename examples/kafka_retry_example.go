package examples

import (
	"context"
	"encoding/json"
	"user-center/internal/worker"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

// Kafka 重试和死信队列使用示例
func KafkaRetryExample(producer sarama.SyncProducer, l logger.Logger) {
	// 1. 定义业务处理器
	businessHandler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		var event map[string]interface{}
		if err := json.Unmarshal(msg.Value, &event); err != nil {
			return err
		}
		
		// 业务逻辑处理
		l.Info("处理消息",
			logger.Field{Key: "topic", Value: msg.Topic},
			logger.Field{Key: "event", Value: event},
		)
		
		// 模拟处理失败
		// return errors.New("处理失败")
		
		return nil
	}

	// 2. 包装为支持重试的处理器
	retryHandler := worker.NewRetryableHandler(
		businessHandler,
		producer,
		l,
	)

	// 3. 在消费者中使用
	handlers := map[string]worker.MessageHandler{
		"user.registered": func(ctx context.Context, msg *sarama.ConsumerMessage) error {
			return retryHandler.Handle(ctx, msg)
		},
		"user.activity": func(ctx context.Context, msg *sarama.ConsumerMessage) error {
			return retryHandler.Handle(ctx, msg)
		},
	}

	consumerHandler := worker.NewConsumerGroupHandler(l, handlers)
	
	// 4. 启动消费者
	// consumerGroup.Consume(ctx, []string{"user.registered", "user.activity"}, consumerHandler)
	
	_ = consumerHandler // 避免未使用警告
}

// 死信队列消费示例
func DeadLetterQueueConsumerExample(l logger.Logger) {
	// 死信队列处理器
	dlqHandler := func(ctx context.Context, msg *sarama.ConsumerMessage) error {
		// 从 header 中获取元数据
		var metadata worker.RetryMetadata
		for _, header := range msg.Headers {
			if string(header.Key) == "dlq_metadata" {
				if err := json.Unmarshal(header.Value, &metadata); err == nil {
					l.Error("死信消息",
						logger.Field{Key: "original_topic", Value: metadata.OriginalTopic},
						logger.Field{Key: "retry_count", Value: metadata.RetryCount},
						logger.Field{Key: "error", Value: metadata.ErrorMessage},
						logger.Field{Key: "first_failed_at", Value: metadata.FirstFailedAt},
					)
				}
			}
		}
		
		// 可以选择：
		// 1. 记录到数据库供人工处理
		// 2. 发送告警通知
		// 3. 尝试特殊处理逻辑
		
		return nil
	}

	handlers := map[string]worker.MessageHandler{
		"user.registered.dlq": dlqHandler,
		"user.activity.dlq":   dlqHandler,
	}

	consumerHandler := worker.NewConsumerGroupHandler(l, handlers)
	
	// 启动死信队列消费者
	// dlqConsumerGroup.Consume(ctx, []string{"user.registered.dlq", "user.activity.dlq"}, consumerHandler)
	
	_ = consumerHandler
}
