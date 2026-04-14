package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client interface {
	Ask(ctx context.Context, req AskRequest) (AskResponse, error)
}

type HTTPClient struct {
	baseURL string
	client  *http.Client
}

type AskRequest struct {
	Query  string `json:"query"`
	TopK   int    `json:"top_k,omitempty"`
	UseLLM *bool  `json:"use_llm,omitempty"`
}

type AskResponse struct {
	Query      string      `json:"query"`
	TopK       int         `json:"top_k"`
	Mode       string      `json:"mode"`
	Answer     string      `json:"answer"`
	References []Reference `json:"references"`
}

type Reference struct {
	DocID      string  `json:"doc_id"`
	Title      string  `json:"title"`
	Source     string  `json:"source"`
	ChunkIndex int     `json:"chunk_index"`
	Score      float64 `json:"score"`
	Content    string  `json:"content"`
}

type ErrorResponse struct {
	Detail any `json:"detail"`
}

func NewHTTPClient(baseURL string, timeout time.Duration) Client {
	return &HTTPClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: timeout,
		},
	}
}

func (c *HTTPClient) Ask(ctx context.Context, req AskRequest) (AskResponse, error) {
	var respData AskResponse

	body, err := json.Marshal(req)
	if err != nil {
		return respData, fmt.Errorf("marshal rag ask request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ask", bytes.NewReader(body))
	if err != nil {
		return respData, fmt.Errorf("build rag ask request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return respData, fmt.Errorf("request rag service: %w", err)
	}
	defer func() {
		_ = httpResp.Body.Close()
	}()

	raw, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return respData, fmt.Errorf("read rag response: %w", err)
	}

	if httpResp.StatusCode >= http.StatusBadRequest {
		var errResp ErrorResponse
		if json.Unmarshal(raw, &errResp) == nil && errResp.Detail != nil {
			return respData, fmt.Errorf("rag service returned %d: %v", httpResp.StatusCode, errResp.Detail)
		}
		return respData, fmt.Errorf("rag service returned %d: %s", httpResp.StatusCode, strings.TrimSpace(string(raw)))
	}

	if err = json.Unmarshal(raw, &respData); err != nil {
		return respData, fmt.Errorf("decode rag response: %w", err)
	}
	return respData, nil
}
