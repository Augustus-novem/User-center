package faliover

import (
	"context"
	"errors"
	"user-center/internal/service/sms"
)

type FailoverSMSService struct {
	svcs []sms.Service

	idx uint64
}

func (f *FailoverSMSService) Send(ctx context.Context, tplId string, args []string, numbers ...string) error {
	for _, svc := range f.svcs {
		err := svc.Send(ctx, tplId, args, numbers...)
		if err == nil {
			return nil
		}
	}
	return errors.New("发送失败，所有服务商都已经尝试过")
}
