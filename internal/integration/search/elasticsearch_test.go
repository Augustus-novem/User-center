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
		case r.Method == http.MethodPut && r.URL.Path == "/notes":
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"type":"resource_already_exists_exception"}}`))
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

func TestElasticsearchSearch(t *testing.T) {
	t.Parallel()
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
			t.Fatalf("body=%+v", body)
		}
		_, _ = w.Write([]byte(`{"hits":{"hits":[{"_source":{"note_id":9,"author_id":3,"title":"Go","content":"search","created_at":123,"status":"published"}}]}}`))
	}))
	defer server.Close()
	client, _ := NewElasticsearch(server.URL, "notes", time.Second)
	notes, err := client.Search(context.Background(), "Go", 7)
	if err != nil || len(notes) != 1 || notes[0].ID != 9 || notes[0].Title != "Go" {
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
