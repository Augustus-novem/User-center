package ioc

import (
	"user-center/internal/config"
	ragclient "user-center/internal/integration/rag"
	"user-center/internal/service"
	"user-center/internal/web"
)

func InitRAGClient(cfg *config.AppConfig) ragclient.Client {
	return ragclient.NewHTTPClient(cfg.RAG.BaseURL, cfg.RAG.Timeout)
}

func InitRAGHandler(svc service.RAGService) *web.RAGHandler {
	return web.NewRAGHandler(svc)
}
