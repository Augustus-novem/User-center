package repository

import "context"

type Transaction interface {
	// InTx 执行事务，如果 fn 返回 error，则回滚；否则提交
	InTx(ctx context.Context, fn func(ctx context.Context) error) error
}
