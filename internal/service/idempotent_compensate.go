package service

import (
	"context"
	"fmt"
	"time"
	"user-center/internal/repository/cache"
	"user-center/pkg/logger"
)

// IdempotentCompensateService 幂等键过期补偿服务
type IdempotentCompensateService interface {
	// CompensateExpiredKeys 补偿过期的幂等键
	CompensateExpiredKeys(ctx context.Context) error
	// CheckAndCompensate 检查并补偿特定业务的幂等键
	CheckAndCompensate(ctx context.Context, businessKey string) error
}

type IdempotentCompensateServiceImpl struct {
	idempotentCache cache.IdempotentCache
	logger          logger.Logger
	// 业务处理记录存储（可以是数据库）
	processRecordRepo ProcessRecordRepository
}

// ProcessRecord 处理记录
type ProcessRecord struct {
	ID            int64
	BusinessKey   string
	ProcessedAt   time.Time
	Status        string // success, failed, processing
	IdempotentKey string
}

// ProcessRecordRepository 处理记录仓储接口
type ProcessRecordRepository interface {
	// FindUncompensated 查找需要补偿的记录（处理成功但幂等键可能过期）
	FindUncompensated(ctx context.Context, before time.Time, limit int) ([]ProcessRecord, error)
	// MarkCompensated 标记已补偿
	MarkCompensated(ctx context.Context, id int64) error
}

func NewIdempotentCompensateService(
	idempotentCache cache.IdempotentCache,
	processRecordRepo ProcessRecordRepository,
	l logger.Logger,
) *IdempotentCompensateServiceImpl {
	return &IdempotentCompensateServiceImpl{
		idempotentCache:   idempotentCache,
		processRecordRepo: processRecordRepo,
		logger:            l,
	}
}

// CompensateExpiredKeys 定期补偿过期的幂等键
func (s *IdempotentCompensateServiceImpl) CompensateExpiredKeys(ctx context.Context) error {
	// 查找最近处理成功但可能幂等键已过期的记录
	// 假设幂等键 TTL 是 24 小时，我们检查 23 小时前的记录
	before := time.Now().Add(-23 * time.Hour)
	records, err := s.processRecordRepo.FindUncompensated(ctx, before, 100)
	if err != nil {
		return fmt.Errorf("查找待补偿记录失败: %w", err)
	}
	
	s.logger.Info("开始补偿幂等键",
		logger.Field{Key: "count", Value: len(records)},
	)
	
	compensated := 0
	for _, record := range records {
		// 检查幂等键是否存在
		exists, err := s.idempotentCache.CheckIdempotentKey(ctx, record.IdempotentKey)
		if err != nil {
			s.logger.Error("检查幂等键失败",
				logger.Field{Key: "business_key", Value: record.BusinessKey},
				logger.Field{Key: "error", Value: err},
			)
			continue
		}
		
		if !exists {
			// 幂等键已过期，重新设置
			_, err = s.idempotentCache.SetIdempotentKey(ctx, record.IdempotentKey, 24*time.Hour)
			if err != nil {
				s.logger.Error("补偿幂等键失败",
					logger.Field{Key: "business_key", Value: record.BusinessKey},
					logger.Field{Key: "error", Value: err},
				)
				continue
			}
			
			s.logger.Info("幂等键补偿成功",
				logger.Field{Key: "business_key", Value: record.BusinessKey},
				logger.Field{Key: "idempotent_key", Value: record.IdempotentKey},
			)
			compensated++
		}
		
		// 标记已补偿
		if err := s.processRecordRepo.MarkCompensated(ctx, record.ID); err != nil {
			s.logger.Error("标记补偿状态失败",
				logger.Field{Key: "record_id", Value: record.ID},
				logger.Field{Key: "error", Value: err},
			)
		}
	}
	
	s.logger.Info("幂等键补偿完成",
		logger.Field{Key: "total", Value: len(records)},
		logger.Field{Key: "compensated", Value: compensated},
	)
	
	return nil
}

// CheckAndCompensate 检查并补偿特定业务的幂等键
func (s *IdempotentCompensateServiceImpl) CheckAndCompensate(ctx context.Context, businessKey string) error {
	idempotentKey := fmt.Sprintf("biz:%s", businessKey)
	
	exists, err := s.idempotentCache.CheckIdempotentKey(ctx, idempotentKey)
	if err != nil {
		return fmt.Errorf("检查幂等键失败: %w", err)
	}
	
	if !exists {
		// 幂等键不存在，重新设置
		_, err = s.idempotentCache.SetIdempotentKey(ctx, idempotentKey, 24*time.Hour)
		if err != nil {
			return fmt.Errorf("补偿幂等键失败: %w", err)
		}
		
		s.logger.Info("幂等键补偿成功",
			logger.Field{Key: "business_key", Value: businessKey},
			logger.Field{Key: "idempotent_key", Value: idempotentKey},
		)
	}
	
	return nil
}
