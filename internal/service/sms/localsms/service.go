package localsms

import (
	"context"
	"user-center/pkg/logger"
)

type Service struct {
	logger logger.Logger
}

func NewService(l logger.Logger) *Service {
	if l == nil {
		l = logger.NewNoOpLogger()
	}
	return &Service{
		logger: l,
	}
}

func (s *Service) Send(ctx context.Context, tplId string,
	args []string, numbers ...string) error {
	s.logger.Info("发送短信验证码（本地模拟）",
		logger.Field{Key: "tplId", Value: tplId},
		logger.Field{Key: "args", Value: args},
		logger.Field{Key: "numbers", Value: numbers},
	)
	return nil
}
