package ratelimit

import (
	"context"
	"errors"
	"fmt"
	"user-center/internal/service/sms"
	"user-center/pkg/ratelimit"
)

const (
	key = "sms_tencent"
)

var (
	errLimited = errors.New("短信服务触发限流")
)

type RatelimitSMSService struct {
	svc     sms.Service
	limiter ratelimit.Limiter
}

func NewRatelimitSMSService(svc sms.Service,
	limiter ratelimit.Limiter) *RatelimitSMSService {
	return &RatelimitSMSService{
		svc:     svc,
		limiter: limiter,
	}
}

func (r *RatelimitSMSService) Send(ctx context.Context,
	tplId string, args []string, numbers ...string) error {
	limited, err := r.limiter.Limit(ctx, key)
	if err != nil {
		return fmt.Errorf("限流工作异常: %w", err)
	}
	if limited {
		return errLimited
	}
	return r.svc.Send(ctx, tplId, args, numbers...)
}
