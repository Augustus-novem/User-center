package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

const (
	// 最大重试次数
	MaxRetryCount = 3
	// 死信队列后缀
	DeadLetterSuffix = ".dlq"
)

// RetryMetadata 重试元数据
type RetryMetadata struct {
	OriginalTopic     string    `json:"original_topic"`
	OriginalPartition int32     `json:"original_partition"`
	OriginalOffset    int64     `json:"original_offset"`
	RetryCount        int       `json:"retry_count"`
	FirstFailedAt     time.Time `json:"first_failed_at"`
	LastFailedAt      time.Time `json:"last_failed_at"`
	ErrorMessage      string    `json:"error_message"`
}

// RetryableHandler 支持重试的消息处理器
type RetryableHandler struct {
	handler  MessageHandler
	producer sarama.SyncProducer
	logger   logger.Logger
}

func NewRetryableHandler(handler MessageHandler, producer sarama.SyncProducer, l logger.Logger) *RetryableHandler {
	return &RetryableHandler{
		handler:  handler,
		producer: producer,
		logger:   l,
	}
}

// Handle 处理消息，失败时自动重试或发送到死信队列
func (h *RetryableHandler) Handle(ctx context.Context, msg *sarama.ConsumerMessage) error {
	// 解析重试元数据
	metadata := h.parseRetryMetadata(msg)
	
	// 执行业务处理
	err := h.handler(ctx, msg)
	if err == nil {
		return nil
	}
	
	// 处理失败，更新元数据
	metadata.RetryCount++
	metadata.LastFailedAt = time.Now()
	metadata.ErrorMessage = err.Error()
	
	if metadata.FirstFailedAt.IsZero() {
		metadata.FirstFailedAt = time.Now()
	}
	
	// 判断是否超过最大重试次数
	if metadata.RetryCount >= MaxRetryCount {
		h.logger.Error("消息重试次数已达上限，发送到死信队列",
			logger.Field{Key: "topic", Value: msg.Topic},
			logger.Field{Key: "partition", Value: msg.Partition},
			logger.Field{Key: "offset", Value: msg.Offset},
			logger.Field{Key: "retry_count", Value: metadata.RetryCount},
			logger.Field{Key: "error", Value: err},
		)
		return h.sendToDeadLetterQueue(msg, metadata)
	}
	
	// 发送到重试队列
	h.logger.Warn("消息处理失败，准备重试",
		logger.Field{Key: "topic", Value: msg.Topic},
		logger.Field{Key: "partition", Value: msg.Partition},
		logger.Field{Key: "offset", Value: msg.Offset},
		logger.Field{Key: "retry_count", Value: metadata.RetryCount},
		logger.Field{Key: "error", Value: err},
	)
	
	return h.sendToRetryQueue(msg, metadata)
}

func (h *RetryableHandler) parseRetryMetadata(msg *sarama.ConsumerMessage) *RetryMetadata {
	for _, header := range msg.Headers {
		if string(header.Key) == "retry_metadata" {
			var metadata RetryMetadata
			if err := json.Unmarshal(header.Value, &metadata); err == nil {
				return &metadata
			}
		}
	}
	
	// 首次处理，创建新的元数据
	return &RetryMetadata{
		OriginalTopic:     msg.Topic,
		OriginalPartition: msg.Partition,
		OriginalOffset:    msg.Offset,
		RetryCount:        0,
	}
}

func (h *RetryableHandler) sendToRetryQueue(msg *sarama.ConsumerMessage, metadata *RetryMetadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("序列化重试元数据失败: %w", err)
	}
	
	// 计算延迟时间（指数退避）
	delay := h.calculateDelay(metadata.RetryCount)
	
	retryMsg := &sarama.ProducerMessage{
		Topic: metadata.OriginalTopic, // 发送回原 topic
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("retry_metadata"),
				Value: metadataBytes,
			},
			{
				Key:   []byte("retry_delay_ms"),
				Value: []byte(fmt.Sprintf("%d", delay.Milliseconds())),
			},
		},
		Timestamp: time.Now().Add(delay),
	}
	
	_, _, err = h.producer.SendMessage(retryMsg)
	if err != nil {
		h.logger.Error("发送重试消息失败",
			logger.Field{Key: "topic", Value: msg.Topic},
			logger.Field{Key: "error", Value: err},
		)
		return err
	}
	
	h.logger.Info("消息已发送到重试队列",
		logger.Field{Key: "topic", Value: metadata.OriginalTopic},
		logger.Field{Key: "retry_count", Value: metadata.RetryCount},
		logger.Field{Key: "delay", Value: delay.String()},
	)
	
	return nil
}

func (h *RetryableHandler) sendToDeadLetterQueue(msg *sarama.ConsumerMessage, metadata *RetryMetadata) error {
	metadataBytes, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("序列化死信元数据失败: %w", err)
	}
	
	dlqTopic := metadata.OriginalTopic + DeadLetterSuffix
	
	dlqMsg := &sarama.ProducerMessage{
		Topic: dlqTopic,
		Key:   sarama.ByteEncoder(msg.Key),
		Value: sarama.ByteEncoder(msg.Value),
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("dlq_metadata"),
				Value: metadataBytes,
			},
		},
	}
	
	_, _, err = h.producer.SendMessage(dlqMsg)
	if err != nil {
		h.logger.Error("发送死信消息失败",
			logger.Field{Key: "dlq_topic", Value: dlqTopic},
			logger.Field{Key: "error", Value: err},
		)
		return err
	}
	
	h.logger.Info("消息已发送到死信队列",
		logger.Field{Key: "original_topic", Value: metadata.OriginalTopic},
		logger.Field{Key: "dlq_topic", Value: dlqTopic},
		logger.Field{Key: "retry_count", Value: metadata.RetryCount},
	)
	
	return nil
}

// calculateDelay 计算重试延迟时间（指数退避）
func (h *RetryableHandler) calculateDelay(retryCount int) time.Duration {
	// 1秒、5秒、30秒
	delays := []time.Duration{
		1 * time.Second,
		5 * time.Second,
		30 * time.Second,
	}
	
	if retryCount > 0 && retryCount <= len(delays) {
		return delays[retryCount-1]
	}
	
	return 30 * time.Second
}
