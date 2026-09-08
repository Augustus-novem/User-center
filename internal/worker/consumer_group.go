package worker

import (
	"context"
	"fmt"
	"strconv"

	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

const DeadLetterSuffix = ".dlq"

type MessageHandler func(ctx context.Context, msg *sarama.ConsumerMessage) error

type DeadLetterPublisher interface {
	SendMessage(msg *sarama.ProducerMessage) (partition int32, offset int64, err error)
}

type ConsumerGroupHandler struct {
	logger   logger.Logger
	dlq      DeadLetterPublisher
	handlers map[string]MessageHandler
}

func NewConsumerGroupHandler(l logger.Logger, handlers map[string]MessageHandler) *ConsumerGroupHandler {
	return &ConsumerGroupHandler{logger: l, handlers: handlers}
}

func NewConsumerGroupHandlerWithDLQ(
	l logger.Logger,
	dlq DeadLetterPublisher,
	handlers map[string]MessageHandler,
) *ConsumerGroupHandler {
	return &ConsumerGroupHandler{logger: l, dlq: dlq, handlers: handlers}
}

func (h *ConsumerGroupHandler) Setup(_ sarama.ConsumerGroupSession) error   { return nil }
func (h *ConsumerGroupHandler) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }

func (h *ConsumerGroupHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case msg, ok := <-claim.Messages():
			if !ok {
				return nil
			}
			handler, exists := h.handlers[msg.Topic]
			if !exists {
				h.logger.Warn("未找到 topic 对应的处理器", logger.Field{Key: "topic", Value: msg.Topic})
				session.MarkMessage(msg, "")
				continue
			}
			err := handler(session.Context(), msg)
			if err == nil {
				session.MarkMessage(msg, "")
				continue
			}
			if IsPermanent(err) {
				if dlqErr := h.publishDLQ(msg, err); dlqErr != nil {
					return fmt.Errorf("publish poison message to DLQ: %w", dlqErr)
				}
				h.logger.Warn("永久非法 Kafka 消息已写入 DLQ",
					logger.Field{Key: "topic", Value: msg.Topic},
					logger.Field{Key: "partition", Value: msg.Partition},
					logger.Field{Key: "offset", Value: msg.Offset},
					logger.Field{Key: "error", Value: err},
				)
				session.MarkMessage(msg, "")
				continue
			}
			h.logger.Error("消费 Kafka 消息失败，当前分区暂停提交后续 offset，等待重试",
				logger.Field{Key: "topic", Value: msg.Topic},
				logger.Field{Key: "partition", Value: msg.Partition},
				logger.Field{Key: "offset", Value: msg.Offset},
				logger.Field{Key: "error", Value: err},
			)
			return fmt.Errorf("topic=%s partition=%d offset=%d: %w", msg.Topic, msg.Partition, msg.Offset, err)
		case <-session.Context().Done():
			return nil
		}
	}
}

func (h *ConsumerGroupHandler) publishDLQ(msg *sarama.ConsumerMessage, cause error) error {
	if h.dlq == nil {
		return fmt.Errorf("DLQ producer is not configured: %w", cause)
	}
	headers := make([]sarama.RecordHeader, 0, len(msg.Headers)+4)
	for _, header := range msg.Headers {
		if header == nil {
			continue
		}
		headers = append(headers, sarama.RecordHeader{
			Key:   append([]byte(nil), header.Key...),
			Value: append([]byte(nil), header.Value...),
		})
	}
	errorText := cause.Error()
	if len(errorText) > 1024 {
		errorText = errorText[:1024]
	}
	headers = append(headers,
		sarama.RecordHeader{Key: []byte("dlq_original_topic"), Value: []byte(msg.Topic)},
		sarama.RecordHeader{Key: []byte("dlq_original_partition"), Value: []byte(strconv.FormatInt(int64(msg.Partition), 10))},
		sarama.RecordHeader{Key: []byte("dlq_original_offset"), Value: []byte(strconv.FormatInt(msg.Offset, 10))},
		sarama.RecordHeader{Key: []byte("dlq_error"), Value: []byte(errorText)},
	)
	_, _, err := h.dlq.SendMessage(&sarama.ProducerMessage{
		Topic:   msg.Topic + DeadLetterSuffix,
		Key:     sarama.ByteEncoder(msg.Key),
		Value:   sarama.ByteEncoder(msg.Value),
		Headers: headers,
	})
	return err
}
