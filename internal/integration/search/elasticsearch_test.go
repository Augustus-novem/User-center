package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
	"user-center/internal/domain"
)

func TestElasticsearchIndexLifecycle(t *testing.T) {
	t.Parallel()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/_alias/notes":
			_, _ = w.Write([]byte(`{"notes-000001":{"aliases":{"notes":{}}}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/notes/_doc/42":
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
				return
			}
			if body["status"] != domain.NoteStatusPublished || body["title"] != "hello" {
				t.Fatalf("unexpected document: %+v", body)
			}
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodDelete && r.URL.Path == "/notes/_doc/42":
			w.WriteHeader(http.StatusNotFound)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	client, err := NewElasticsearch(server.URL, "notes", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err = client.EnsureIndex(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = client.Index(context.Background(), domain.Note{ID: 42, Title: "hello", Status: domain.NoteStatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err = client.Delete(context.Background(), 42); err != nil {
		t.Fatal(err)
	}
	if len(requests) != 3 {
		t.Fatalf("requests=%v", requests)
	}
}

func TestElasticsearchRebuildAtomicallySwapsAliasAndDeletesOldIndex(t *testing.T) {
	t.Parallel()
	var physical string
	var aliasActions map[string]any
	oldDeleted := false
	runtimeWrite := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/notes-rebuild-") && !strings.Contains(r.URL.Path, "/_doc/"):
			physical = strings.TrimPrefix(r.URL.Path, "/")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/"+physical+"/_doc/1":
			w.WriteHeader(http.StatusCreated)
		case r.Method == http.MethodGet && r.URL.Path == "/_alias/notes":
			_, _ = w.Write([]byte(`{"notes-old":{"aliases":{"notes":{}}}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/_aliases":
			if err := json.NewDecoder(r.Body).Decode(&aliasActions); err != nil {
				t.Errorf("decode alias actions: %v", err)
			}
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodDelete && r.URL.Path == "/notes-old":
			oldDeleted = true
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/notes/_doc/2":
			runtimeWrite = true
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewElasticsearch(server.URL, "notes", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	physical, err = client.BeginRebuild(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = client.IndexInto(context.Background(), physical, domain.Note{ID: 1, Status: domain.NoteStatusPublished}); err != nil {
		t.Fatal(err)
	}
	if err = client.CommitRebuild(context.Background(), physical); err != nil {
		t.Fatal(err)
	}
	if aliasActions == nil || !oldDeleted {
		t.Fatalf("aliasActions=%+v oldDeleted=%v", aliasActions, oldDeleted)
	}
	actionsJSON, err := json.Marshal(aliasActions)
	if err != nil {
		t.Fatal(err)
	}
	actionsText := string(actionsJSON)
	if !strings.Contains(actionsText, `"remove":{"alias":"notes","index":"notes-old"}`) ||
		!strings.Contains(actionsText, `"add":{"alias":"notes","index":"`+physical+`"}`) {
		t.Fatalf("alias swap must atomically remove old and add new target: %s", actionsText)
	}
	if err = client.Index(context.Background(), domain.Note{ID: 2, Status: domain.NoteStatusPublished}); err != nil {
		t.Fatal(err)
	}
	if !runtimeWrite {
		t.Fatal("runtime write must continue targeting alias")
	}
}

func TestElasticsearchSearch(t *testing.T) {
	t.Parallel()
	largeContent := strings.Repeat("x", 5000)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/notes/_search" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if body["size"].(float64) != 7 {
			t.Errorf("body=%+v", body)
			return
		}
		response := map[string]any{
			"hits": map[string]any{
				"hits": []any{
					map[string]any{"_source": map[string]any{
						"note_id": 9, "author_id": 3, "title": "Go", "content": largeContent,
						"created_at": 123, "status": "published",
					}},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	client, _ := NewElasticsearch(server.URL, "notes", time.Second)
	notes, err := client.Search(context.Background(), "Go", 7)
	if err != nil || len(notes) != 1 || notes[0].ID != 9 || notes[0].Title != "Go" || len(notes[0].Content) != 5000 {
		t.Fatalf("notes=%+v err=%v", notes, err)
	}
}

func TestElasticsearchTimeout(t *testing.T) {
	t.Parallel()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer server.Close()
	client, _ := NewElasticsearch(server.URL, "notes", 20*time.Millisecond)
	_, err := client.Search(context.Background(), "Go", 1)
	if err == nil || !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("expected timeout, got %v", err)
	}
}
