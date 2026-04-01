package dao

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

type GORMTransaction struct {
	db *gorm.DB
}

func NewGORMTransaction(db *gorm.DB) *GORMTransaction {
	return &GORMTransaction{db: db}
}

func (g *GORMTransaction) InTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return g.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		txCtx := context.WithValue(ctx, txKey{}, tx)
		return fn(txCtx)
	})
}

func dbFromCtx(ctx context.Context, db *gorm.DB) *gorm.DB {
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	if ok && tx != nil {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}
