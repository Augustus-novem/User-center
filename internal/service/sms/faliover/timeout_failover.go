package faliover

import (
	"context"
	"errors"
	"sync/atomic"
	"user-center/internal/service/sms"
)

type TimeoutFailoverSMSService struct {
	svcs      []sms.Service
	idx       int32
	cnt       int32 // 连续超时次数
	threshold int32 // 连续超时次数阈值
}

func NewTimeoutFailoverSMSService(svcs []sms.Service, threshold int32) *TimeoutFailoverSMSService {
	return &TimeoutFailoverSMSService{
		svcs:      svcs,
		threshold: threshold,
	}
}

func (t *TimeoutFailoverSMSService) Send(ctx context.Context, tplId string, args []string, numbers ...string) error {
	cnt := atomic.LoadInt32(&t.cnt)
	idx := atomic.LoadInt32(&t.idx)
	if cnt >= t.threshold {
		newIdx := (idx + 1) % int32(len(t.svcs))
		if atomic.CompareAndSwapInt32(&t.idx, idx, newIdx) {
			atomic.StoreInt32(&t.cnt, 0)
		}
		idx = newIdx
	}
	svc := t.svcs[idx]
	err := svc.Send(ctx, tplId, args, numbers...)
	switch {
	case err == nil:
		// 没有任何错误，重置计数器
		atomic.StoreInt32(&t.cnt, 0)
	case errors.Is(err, context.DeadlineExceeded):
		atomic.AddInt32(&t.cnt, 1)
	default:
		// 如果是别的异常的话，我们保持不动
	}
	return err
}
