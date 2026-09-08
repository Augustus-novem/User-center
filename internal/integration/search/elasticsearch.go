package search

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"user-center/internal/domain"
)

const (
	maxErrorBody    = 4096
	maxResponseBody = 2 * 1024 * 1024
)

type Elasticsearch struct {
	address string
	index   string
	client  *http.Client
}

func NewElasticsearch(address, index string, timeout time.Duration) (*Elasticsearch, error) {
	address = strings.TrimRight(strings.TrimSpace(address), "/")
	parsed, err := url.Parse(address)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("invalid Elasticsearch address %q", address)
	}
	if strings.TrimSpace(index) == "" {
		return nil, errors.New("Elasticsearch index is empty")
	}
	if timeout <= 0 {
		return nil, errors.New("Elasticsearch timeout must be positive")
	}
	return &Elasticsearch{
		address: address,
		index:   index,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (e *Elasticsearch) EnsureIndex(ctx context.Context) error {
	body := map[string]any{
		"mappings": map[string]any{
			"dynamic": "strict",
			"properties": map[string]any{
				"note_id":    map[string]string{"type": "long"},
				"author_id":  map[string]string{"type": "long"},
				"title":      map[string]string{"type": "text"},
				"content":    map[string]string{"type": "text"},
				"created_at": map[string]string{"type": "date", "format": "epoch_millis"},
				"status":     map[string]string{"type": "keyword"},
			},
		},
	}
	status, response, err := e.doJSON(ctx, http.MethodPut, e.indexPath(), body)
	if err != nil {
		return err
	}
	if status >= 200 && status < 300 {
		return nil
	}
	if status == http.StatusBadRequest && bytes.Contains(response, []byte("resource_already_exists_exception")) {
		return nil
	}
	return responseError(http.MethodPut, status, response)
}

func (e *Elasticsearch) Index(ctx context.Context, note domain.Note) error {
	document := map[string]any{
		"note_id": note.ID, "author_id": note.AuthorID,
		"title": note.Title, "content": note.Content,
		"created_at": note.CreatedAt, "status": note.Status,
	}
	status, response, err := e.doJSON(ctx, http.MethodPut, e.documentPath(note.ID), document)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return responseError(http.MethodPut, status, response)
	}
	return nil
}

func (e *Elasticsearch) Delete(ctx context.Context, noteID int64) error {
	status, response, err := e.doJSON(ctx, http.MethodDelete, e.documentPath(noteID), nil)
	if err != nil {
		return err
	}
	if (status >= 200 && status < 300) || status == http.StatusNotFound {
		return nil
	}
	return responseError(http.MethodDelete, status, response)
}

func (e *Elasticsearch) Search(ctx context.Context, query string, limit int) ([]domain.Note, error) {
	body := map[string]any{
		"size":    limit,
		"_source": []string{"note_id", "author_id", "title", "content", "created_at", "status"},
		"query": map[string]any{"bool": map[string]any{
			"must":   []any{map[string]any{"multi_match": map[string]any{"query": query, "fields": []string{"title^2", "content"}}}},
			"filter": []any{map[string]any{"term": map[string]string{"status": domain.NoteStatusPublished}}},
		}},
		"sort": []any{"_score", map[string]any{"created_at": "desc"}, map[string]any{"note_id": "desc"}},
	}
	status, response, err := e.doJSON(ctx, http.MethodPost, e.indexPath()+"/_search", body)
	if err != nil {
		return nil, err
	}
	if status < 200 || status >= 300 {
		return nil, responseError(http.MethodPost, status, response)
	}
	var result struct {
		Hits struct {
			Hits []struct {
				Source struct {
					NoteID    int64  `json:"note_id"`
					AuthorID  int64  `json:"author_id"`
					Title     string `json:"title"`
					Content   string `json:"content"`
					CreatedAt int64  `json:"created_at"`
					Status    string `json:"status"`
				} `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err = json.Unmarshal(response, &result); err != nil {
		return nil, fmt.Errorf("decode Elasticsearch search response: %w", err)
	}
	notes := make([]domain.Note, 0, len(result.Hits.Hits))
	for _, hit := range result.Hits.Hits {
		notes = append(notes, domain.Note{
			ID: hit.Source.NoteID, AuthorID: hit.Source.AuthorID,
			Title: hit.Source.Title, Content: hit.Source.Content,
			CreatedAt: hit.Source.CreatedAt, Status: hit.Source.Status,
		})
	}
	return notes, nil
}

func (e *Elasticsearch) doJSON(ctx context.Context, method, path string, payload any) (int, []byte, error) {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return 0, nil, err
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, e.address+path, body)
	if err != nil {
		return 0, nil, err
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := e.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("Elasticsearch %s request: %w", method, err)
	}
	defer func() { _ = resp.Body.Close() }()
	response, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return 0, nil, fmt.Errorf("read Elasticsearch response: %w", err)
	}
	if len(response) > maxResponseBody {
		return 0, nil, fmt.Errorf("Elasticsearch response exceeds %d bytes", maxResponseBody)
	}
	return resp.StatusCode, response, nil
}

func (e *Elasticsearch) indexPath() string {
	return "/" + url.PathEscape(e.index)
}

func (e *Elasticsearch) documentPath(noteID int64) string {
	return e.indexPath() + "/_doc/" + strconv.FormatInt(noteID, 10)
}

func responseError(method string, status int, body []byte) error {
	if len(body) > maxErrorBody {
		body = body[:maxErrorBody]
	}
	return fmt.Errorf("Elasticsearch %s returned status %d: %s", method, status, strings.TrimSpace(string(body)))
}
