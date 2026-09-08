package main

import (
	"context"
	"user-center/internal/repository"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/ioc"

	"go.uber.org/zap"
)

func main() {
	cfgManager, err := ioc.InitConfig()
	if err != nil {
		panic(err)
	}
	cfg := cfgManager.App()
	if !cfg.Search.Enabled {
		panic("search.enabled 必须为 true")
	}
	zapLogger, _, err := ioc.InitLogger(cfg.Log)
	if err != nil {
		panic(err)
	}
	defer func() { _ = zapLogger.Sync() }()
	db := ioc.InitDB(&cfg)
	notes := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	index := ioc.InitNoteSearchIndex(&cfg)
	indexer := service.NewSearchIndexService(notes, notes, index, cfg.Search.ReindexBatchSize)
	count, err := indexer.Rebuild(context.Background())
	if err != nil {
		panic(err)
	}
	zapLogger.Info("搜索索引重建完成", zap.Int("indexed_notes", count))
}
