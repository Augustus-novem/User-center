//go:build e2e

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
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
	apiResult := doJSON(t, httpServer.URL, http.MethodGet, "/search/notes?q="+url.QueryEscape(keyword), token, nil)
	if apiResult.Code != 0 || degradedFromData(apiResult.Data) {
		t.Fatalf("search API should use Elasticsearch: %+v", apiResult)
	}

	updatedKeyword := "updated" + strings.ReplaceAll(uuid.NewString(), "-", "")
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

type failingRebuildIndex struct {
	repository.NoteSearchIndex
	failOnCall int
	calls      int
}

func (f *failingRebuildIndex) IndexInto(ctx context.Context, physical string, note domain.Note) error {
	f.calls++
	if f.calls == f.failOnCall {
		return errors.New("injected rebuild failure")
	}
	return f.NoteSearchIndex.IndexInto(ctx, physical, note)
}

func TestSearch_RebuildAliasLifecycleE2E(t *testing.T) {
	cfg := loadE2EConfig(t)
	cfg.Search.Enabled = true
	cfg.Search.Index = "community_notes_rebuild_e2e_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	pingE2EDeps(t, cfg)

	index := ioc.InitNoteSearchIndex(&cfg)
	if err := index.EnsureIndex(context.Background()); err != nil {
		t.Skipf("Elasticsearch not available: %v", err)
	}
	t.Cleanup(func() { deleteAliasTargets(t, cfg.Search.Address, cfg.Search.Index) })

	db := ioc.InitDB(&cfg)
	notes := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	seed, err := notes.Create(context.Background(), domain.Note{
		AuthorID: 1, Title: "rebuild seed " + uuid.NewString(), Content: "published",
		Status: domain.NoteStatusPublished,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Unscoped().Where("id = ?", seed.ID).Delete(&dao.NoteOfDB{}).Error })

	markerID := time.Now().UnixNano()
	markerKeyword := "old-alias-marker-" + uuid.NewString()
	if err = index.Index(context.Background(), domain.Note{
		ID: markerID, AuthorID: 1, Title: markerKeyword, Content: markerKeyword,
		Status: domain.NoteStatusPublished, CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	waitForSearch(t, 10*time.Second, func() (bool, error) {
		items, searchErr := index.Search(context.Background(), markerKeyword, 10)
		return containsSearchNote(items, markerID), searchErr
	})

	failing := &failingRebuildIndex{NoteSearchIndex: index, failOnCall: 1}
	failingService := service.NewSearchIndexService(notes, notes, failing, cfg.Search.ReindexBatchSize)
	if _, err = failingService.Rebuild(context.Background()); err == nil {
		t.Fatal("injected rebuild should fail")
	}
	items, err := index.Search(context.Background(), markerKeyword, 10)
	if err != nil || !containsSearchNote(items, markerID) {
		t.Fatalf("failed rebuild must leave old alias available: items=%+v err=%v", items, err)
	}

	indexer := service.NewSearchIndexService(notes, notes, index, cfg.Search.ReindexBatchSize)
	if _, err = indexer.Rebuild(context.Background()); err != nil {
		t.Fatalf("successful rebuild: %v", err)
	}
	items, err = index.Search(context.Background(), markerKeyword, 10)
	if err != nil || containsSearchNote(items, markerID) {
		t.Fatalf("stale ES-only document survived rebuild: items=%+v err=%v", items, err)
	}
	targets := aliasTargets(t, cfg.Search.Address, cfg.Search.Index)
	if len(targets) != 1 || targets[0] == cfg.Search.Index {
		t.Fatalf("alias must point to one physical rebuild index: %v", targets)
	}

	newID := markerID + 1
	newKeyword := "post-swap-write-" + uuid.NewString()
	if err = index.Index(context.Background(), domain.Note{
		ID: newID, AuthorID: 1, Title: newKeyword, Content: newKeyword,
		Status: domain.NoteStatusPublished, CreatedAt: time.Now().UnixMilli(),
	}); err != nil {
		t.Fatal(err)
	}
	waitForSearch(t, 10*time.Second, func() (bool, error) {
		items, searchErr := index.Search(context.Background(), newKeyword, 10)
		return containsSearchNote(items, newID), searchErr
	})
}

func aliasTargets(t *testing.T, address, alias string) []string {
	t.Helper()
	resp, err := http.Get(strings.TrimRight(address, "/") + "/_alias/" + url.PathEscape(alias))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		t.Fatalf("load alias %s: status=%d", alias, resp.StatusCode)
	}
	var result map[string]json.RawMessage
	if err = json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	targets := make([]string, 0, len(result))
	for name := range result {
		targets = append(targets, name)
	}
	return targets
}

func deleteAliasTargets(t *testing.T, address, alias string) {
	t.Helper()
	for _, target := range aliasTargets(t, address, alias) {
		req, err := http.NewRequest(http.MethodDelete, strings.TrimRight(address, "/")+"/"+url.PathEscape(target), nil)
		if err != nil {
			continue
		}
		if resp, requestErr := http.DefaultClient.Do(req); requestErr == nil {
			_ = resp.Body.Close()
		}
	}
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
