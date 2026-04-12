package worker

import (
	"context"
	"fmt"

	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type MessageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) error

type ConsumerGroupHandler struct {
	logger   logger.Logger
	handlers map[string]MessageHandler
}

func NewConsumerGroupHandler(l logger.Logger, handlers map[string]MessageHandler) *ConsumerGroupHandler {
	return &ConsumerGroupHandler{logger: l, handlers: handlers}
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error {
	return nil
}

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			handler, exists := h.handlers[msg.Topic]
			if !exists {
				h.logger.Warn("未找到 topic 对应的处理器",
					logger.Field{Key: "topic", Value: msg.Topic},
				)
				session.MarkMessage(msg, "")
				continue
			}
			if err := handler(session.Context(), msg); err != nil {
				h.logger.Error("消费 Kafka 消息失败，当前分区暂停提交后续 offset，等待重试",
					logger.Field{Key: "topic", Value: msg.Topic},
					logger.Field{Key: "partition", Value: msg.Partition},
					logger.Field{Key: "offset", Value: msg.Offset},
					logger.Field{Key: "error", Value: err},
				)
				return fmt.Errorf("topic=%s partition=%d offset=%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
			}
			session.MarkMessage(msg, "")
		case <-session.Context().Done():
			return nil
		}
	}
}
