package web

import (
	"errors"
	"strings"
	"user-center/internal/service"

	"github.com/gin-gonic/gin"
)

type RAGHandler struct {
	svc service.RAGService
}

func NewRAGHandler(svc service.RAGService) *RAGHandler {
	return &RAGHandler{svc: svc}
}

func (h *RAGHandler) RegisterRoutes(server *gin.Engine) {
	g := server.Group("/rag")
	g.POST("/ask", h.Ask)
}

func (h *RAGHandler) Ask(ctx *gin.Context) {
	type Req struct {
		Query  string `json:"query"`
		TopK   int    `json:"top_k"`
		UseLLM *bool  `json:"use_llm"`
	}
	var req Req
	if err := ctx.ShouldBindJSON(&req); err != nil {
		JSONBadRequest(ctx, "请求参数错误")
		return
	}
	req.Query = strings.TrimSpace(req.Query)
	if req.Query == "" {
		JSONBadRequest(ctx, "query 不能为空")
		return
	}

	res, err := h.svc.Ask(ctx.Request.Context(), service.RAGAskRequest{
		Query:  req.Query,
		TopK:   req.TopK,
		UseLLM: req.UseLLM,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrRAGDisabled):
			JSONBizError(ctx, "RAG 功能未开启")
		case strings.Contains(err.Error(), "query cannot be empty"):
			JSONBadRequest(ctx, "query 不能为空")
		default:
			JSONInternalServerError(ctx, "知识库问答服务不可用")
		}
		return
	}

	refs := make([]gin.H, 0, len(res.References))
	for _, ref := range res.References {
		refs = append(refs, gin.H{
			"doc_id":      ref.DocID,
			"title":       ref.Title,
			"source":      ref.Source,
			"chunk_index": ref.ChunkIndex,
			"score":       ref.Score,
			"content":     ref.Content,
		})
	}

	JSONOK(ctx, "问答成功", gin.H{
		"query":      res.Query,
		"top_k":      res.TopK,
		"mode":       res.Mode,
		"answer":     res.Answer,
		"references": refs,
	})
}
