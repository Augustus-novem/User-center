package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"user-center/internal/config"
	ragclient "user-center/internal/integration/rag"
)

var ErrRAGDisabled = errors.New("rag is disabled")

//go:generate mockgen -source=rag.go -destination=../service/mocks/rag.mock.go -package=svcmocks

type RAGService interface {
	Ask(ctx context.Context, req RAGAskRequest) (RAGAskResult, error)
}

type RAGAskRequest struct {
	Query  string
	TopK   int
	UseLLM *bool
}

type RAGAskResult struct {
	Query      string
	TopK       int
	Mode       string
	Answer     string
	References []RAGReference
}

type RAGReference struct {
	DocID      string
	Title      string
	Source     string
	ChunkIndex int
	Score      float64
	Content    string
}

type RAGServiceImpl struct {
	client ragclient.Client
	cfg    config.RAGConfig
}

func NewRAGServiceImpl(client ragclient.Client, cfg *config.AppConfig) *RAGServiceImpl {
	return &RAGServiceImpl{
		client: client,
		cfg:    cfg.RAG,
	}
}

func (s *RAGServiceImpl) Ask(ctx context.Context, req RAGAskRequest) (RAGAskResult, error) {
	var result RAGAskResult

	query := strings.TrimSpace(req.Query)
	if query == "" {
		return result, fmt.Errorf("query cannot be empty")
	}
	if !s.cfg.Enabled {
		return result, ErrRAGDisabled
	}

	topK := req.TopK
	if topK <= 0 {
		topK = s.cfg.DefaultTopK
	}
	if topK <= 0 {
		topK = 3
	}

	useLLM := req.UseLLM
	if useLLM == nil {
		defaultUseLLM := s.cfg.UseLLM
		useLLM = &defaultUseLLM
	}

	resp, err := s.client.Ask(ctx, ragclient.AskRequest{
		Query:  query,
		TopK:   topK,
		UseLLM: useLLM,
	})
	if err != nil {
		return result, err
	}

	refs := make([]RAGReference, 0, len(resp.References))
	for _, ref := range resp.References {
		refs = append(refs, RAGReference{
			DocID:      ref.DocID,
			Title:      ref.Title,
			Source:     ref.Source,
			ChunkIndex: ref.ChunkIndex,
			Score:      ref.Score,
			Content:    ref.Content,
		})
	}

	result = RAGAskResult{
		Query:      resp.Query,
		TopK:       resp.TopK,
		Mode:       resp.Mode,
		Answer:     resp.Answer,
		References: refs,
	}
	if result.Query == "" {
		result.Query = query
	}
	if result.TopK <= 0 {
		result.TopK = topK
	}
	return result, nil
}
