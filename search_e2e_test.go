//go:build e2e

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/worker"
	"user-center/ioc"
	"user-center/pkg/logger"

	"github.com/google/uuid"
)

func TestSearch_NoteLifecycleThroughKafkaE2E(t *testing.T) {
	cfg := loadE2EConfig(t)
	cfg.Search.Enabled = true
	cfg.Search.Index = "community_notes_e2e_" + uuid.NewString()
	cfg.Search.ConsumerGroup = "user-center-search-e2e-" + uuid.NewString()
	pingE2EDeps(t, cfg)

	index := ioc.InitNoteSearchIndex(&cfg)
	if err := index.EnsureIndex(context.Background()); err != nil {
		t.Skipf("Elasticsearch not available: %v", err)
	}
	t.Cleanup(func() {
		req, _ := http.NewRequest(http.MethodDelete, cfg.Search.Address+"/"+url.PathEscape(cfg.Search.Index), nil)
		if req != nil {
			resp, err := http.DefaultClient.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	})

	appLogger := logger.NewNoOpLogger()
	if err := ioc.EnsureKafkaTopics(&cfg, appLogger); err != nil {
		t.Fatalf("ensure topics: %v", err)
	}
	db := ioc.InitDB(&cfg)
	notes := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	indexer := service.NewSearchIndexService(notes, notes, index, cfg.Search.ReindexBatchSize)
	searchHandler := worker.NewSearchIndexHandler(indexer, appLogger)
	group := ioc.InitSearchKafkaConsumerGroup(&cfg)
	t.Cleanup(func() { _ = group.Close() })
	consumeCtx, consumeCancel := context.WithCancel(context.Background())
	t.Cleanup(consumeCancel)
	go func() {
		_ = group.Consume(consumeCtx, []string{events.TopicNotePublished, events.TopicNoteDeleted}, worker.NewConsumerGroupHandler(appLogger, map[string]worker.MessageHandler{
			events.TopicNotePublished: searchHandler.HandlePublished,
			events.TopicNoteDeleted:   searchHandler.HandleDeleted,
		}))
	}()

	relay := ioc.InitEventRelay(&cfg, db, appLogger)
	if relay == nil {
		t.Fatal("outbox relay is nil")
	}
	t.Cleanup(func() { _ = relay.Close() })
	relayCtx, relayCancel := context.WithCancel(context.Background())
	t.Cleanup(relayCancel)
	go relay.Run(relayCtx, 100*time.Millisecond)

	server := InitWebServer(&cfg, staticDynamic{cfg: cfg}, appLogger)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	suffix := uuid.NewString()
	token := signupAndLogin(t, httpServer.URL, suffix+"@search-e2e.local", "Passw0rd!")
	keyword := "search-e2e-" + suffix
	published := doJSON(t, httpServer.URL, http.MethodPost, "/notes", token, map[string]any{
		"title":   keyword,
		"content": "Kafka to Elasticsearch",
	})
	if published.Code != 0 {
		t.Fatalf("publish: %+v", published)
	}
	noteID := intFromData(t, published.Data, "id")

	waitForSearch(t, 30*time.Second, func() (bool, error) {
		items, err := index.Search(context.Background(), keyword, 10)
		return containsSearchNote(items, noteID), err
	})
	apiResult := doJSON(t, httpServer.URL, http.MethodGet, "/search/notes?q="+url.QueryEscape(keyword), "", nil)
	if apiResult.Code != 0 || degradedFromData(apiResult.Data) {
		t.Fatalf("search API should use Elasticsearch: %+v", apiResult)
	}

	updatedKeyword := keyword + "-updated"
	if err := index.Index(context.Background(), domain.Note{
		ID: noteID, AuthorID: profileID(t, httpServer.URL, token),
		Title: updatedKeyword, Content: "updated adapter document",
		Status: domain.NoteStatusPublished, CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatalf("update indexed document: %v", err)
	}
	waitForSearch(t, 30*time.Second, func() (bool, error) {
		items, err := index.Search(context.Background(), updatedKeyword, 10)
		return containsSearchNote(items, noteID), err
	})

	deleted := doJSON(t, httpServer.URL, http.MethodDelete, fmt.Sprintf("/notes/%d", noteID), token, nil)
	if deleted.Code != 0 {
		t.Fatalf("delete: %+v", deleted)
	}
	waitForSearch(t, 30*time.Second, func() (bool, error) {
		items, err := index.Search(context.Background(), updatedKeyword, 10)
		return !containsSearchNote(items, noteID), err
	})
}

func waitForSearch(t *testing.T, timeout time.Duration, check func() (bool, error)) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		ok, err := check()
		if err == nil && ok {
			return
		}
		lastErr = err
		timer := time.NewTimer(200 * time.Millisecond)
		<-timer.C
	}
	t.Fatalf("timed out waiting for Elasticsearch, last error: %v", lastErr)
}

func containsSearchNote(notes []domain.Note, noteID int64) bool {
	for _, note := range notes {
		if note.ID == noteID {
			return true
		}
	}
	return false
}

func degradedFromData(data any) bool {
	object, ok := data.(map[string]any)
	if !ok {
		return false
	}
	degraded, _ := object["degraded"].(bool)
	return degraded
}
