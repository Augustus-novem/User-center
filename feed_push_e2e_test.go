//go:build e2e

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"user-center/internal/config"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/web"
	"user-center/internal/worker"
	"user-center/ioc"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const e2eUserAgent = "m05-push-feed-e2e"

func TestPushFeed_NotePublishedFanoutE2E(t *testing.T) {
	cfg := loadE2EConfig(t)
	cfg.Kafka.ConsumerGroup = "user-center-e2e-m05"
	cfg.Kafka.ClientID = "user-center-e2e-m05"
	pingE2EDeps(t, cfg)

	appLogger := logger.NewNoOpLogger()
	if err := ioc.EnsureKafkaTopics(&cfg, appLogger); err != nil {
		t.Fatalf("ensure topics: %v", err)
	}

	server := InitWebServer(&cfg, staticDynamic{cfg: cfg}, appLogger)
	ts := httptest.NewServer(server)
	t.Cleanup(ts.Close)

	db := ioc.InitDB(&cfg)
	relay := ioc.InitEventRelay(&cfg, db, appLogger)
	if relay == nil {
		t.Fatal("outbox relay is nil")
	}
	t.Cleanup(func() { _ = relay.Close() })
	relayCtx, relayCancel := context.WithCancel(context.Background())
	t.Cleanup(relayCancel)
	go relay.Run(relayCtx, 200*time.Millisecond)

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	t.Cleanup(func() { _ = rdb.Close() })

	followRepo := repository.NewFollowRepositoryImpl(dao.NewGORMFollowDAO(db))
	noteRepo := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	inbox := cache.NewRedisFeedInbox(rdb, cfg.Feed.InboxLimit())
	feedSvc := service.NewFeedServiceImpl(inbox, noteRepo, followRepo, cfg.Feed.FanoutBatch(), cfg.Feed.CelebrityThreshold(), appLogger)
	handler := worker.NewNotePublishedHandler(
		feedSvc,
		worker.NewRedisDeduplicator(rdb, worker.NotePublishedDeduperNamespace),
		appLogger,
	)

	group := ioc.InitKafkaConsumerGroup(&cfg)
	t.Cleanup(func() { _ = group.Close() })
	consumeCtx, consumeCancel := context.WithCancel(context.Background())
	t.Cleanup(consumeCancel)
	go func() {
		_ = group.Consume(consumeCtx, []string{events.TopicNotePublished}, worker.NewConsumerGroupHandler(appLogger, map[string]worker.MessageHandler{
			events.TopicNotePublished: handler.Handle,
		}))
	}()
	time.Sleep(2 * time.Second)

	suffix := time.Now().UnixNano()
	password := "Passw0rd!"
	emailA := fmt.Sprintf("a%d@e2e.local", suffix)
	emailB := fmt.Sprintf("b%d@e2e.local", suffix)
	tokenA := signupAndLogin(t, ts.URL, emailA, password)
	tokenB := signupAndLogin(t, ts.URL, emailB, password)
	idA := profileID(t, ts.URL, tokenA)
	idB := profileID(t, ts.URL, tokenB)

	followResp := doJSON(t, ts.URL, http.MethodPost, fmt.Sprintf("/users/%d/follow", idA), tokenB, nil)
	if followResp.Code != 0 {
		t.Fatalf("follow: %+v", followResp)
	}

	pub := doJSON(t, ts.URL, http.MethodPost, "/notes", tokenA, map[string]any{
		"title":   "e2e push feed",
		"content": "from A",
	})
	if pub.Code != 0 {
		t.Fatalf("publish: %+v", pub)
	}
	noteID := intFromData(t, pub.Data, "id")

	waitUntil(t, 20*time.Second, func() bool {
		feed := doJSON(t, ts.URL, http.MethodGet, "/feed/following", tokenB, nil)
		items := itemsFromData(feed.Data)
		return feed.Code == 0 && len(items) == 1 && intFromMap(items[0], "id") == noteID
	})

	card, err := rdb.ZCard(context.Background(), cache.FeedInboxKey(idB)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if card != 1 {
		t.Fatalf("inbox card=%d want 1", card)
	}

	evt, err := latestNotePublished(db)
	if err != nil {
		t.Fatalf("load outbox event: %v", err)
	}
	payload, err := json.Marshal(evt)
	if err != nil {
		t.Fatal(err)
	}
	if err = handler.Handle(context.Background(), &sarama.ConsumerMessage{Value: payload}); err != nil {
		t.Fatalf("replay: %v", err)
	}
	feed := doJSON(t, ts.URL, http.MethodGet, "/feed/following", tokenB, nil)
	items := itemsFromData(feed.Data)
	if feed.Code != 0 || len(items) != 1 || intFromMap(items[0], "id") != noteID {
		t.Fatalf("replay must not duplicate feed, got %+v", feed)
	}
	card, err = rdb.ZCard(context.Background(), cache.FeedInboxKey(idB)).Result()
	if err != nil {
		t.Fatal(err)
	}
	if card != 1 {
		t.Fatalf("replay must not duplicate inbox, card=%d", card)
	}
}

type staticDynamic struct {
	cfg config.AppConfig
}

func (s staticDynamic) Dynamic() config.DynamicConfig {
	return config.DynamicConfig{Feature: s.cfg.Feature}
}

func loadE2EConfig(t *testing.T) config.AppConfig {
	t.Helper()
	mgr, err := config.NewManager("config/dev.yaml")
	if err != nil {
		t.Skipf("load config: %v", err)
	}
	return mgr.App()
}

func pingE2EDeps(t *testing.T, cfg config.AppConfig) {
	t.Helper()
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	defer func() { _ = rdb.Close() }()
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	sqlDB, err := gorm.Open(mysql.Open(cfg.DB.DSN))
	if err != nil {
		t.Skipf("mysql not available: %v", err)
	}
	raw, err := sqlDB.DB()
	if err != nil {
		t.Skipf("mysql db: %v", err)
	}
	if err = raw.Ping(); err != nil {
		t.Skipf("mysql ping: %v", err)
	}
	_ = raw.Close()
	if !cfg.Kafka.Enabled {
		t.Skip("kafka disabled")
	}
	client, err := sarama.NewClient(cfg.Kafka.Brokers, sarama.NewConfig())
	if err != nil {
		t.Skipf("kafka not available: %v", err)
	}
	_ = client.Close()
}

func signupAndLogin(t *testing.T, base, email, password string) string {
	t.Helper()
	signup := doJSON(t, base, http.MethodPost, "/user/signup", "", map[string]any{
		"email":              email,
		"password":           password,
		"confirmed_password": password,
	})
	if signup.Code != 0 {
		t.Fatalf("signup %s: %+v", email, signup)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/user/login", mustJSON(map[string]any{
		"email":    email,
		"password": password,
	}))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", e2eUserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	var parsed web.Result
	_ = json.Unmarshal(body, &parsed)
	if parsed.Code != 0 {
		t.Fatalf("login %s: %+v", email, parsed)
	}
	token := resp.Header.Get("x-jwt-token")
	if token == "" {
		t.Fatal("missing x-jwt-token")
	}
	return token
}

func profileID(t *testing.T, base, token string) int64 {
	t.Helper()
	res := doJSON(t, base, http.MethodGet, "/user/profile", token, nil)
	if res.Code != 0 {
		t.Fatalf("profile: %+v", res)
	}
	return intFromData(t, res.Data, "id")
}

func doJSON(t *testing.T, base, method, path, token string, payload any) web.Result {
	t.Helper()
	var body io.Reader
	if payload != nil {
		body = mustJSON(payload)
	}
	req, err := http.NewRequest(method, base+path, body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("User-Agent", e2eUserAgent)
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	var parsed web.Result
	if err = json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode %s %s status=%d body=%s: %v", method, path, resp.StatusCode, raw, err)
	}
	return parsed
}

func mustJSON(v any) *bytes.Buffer {
	bs, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return bytes.NewBuffer(bs)
}

func intFromData(t *testing.T, data any, key string) int64 {
	t.Helper()
	m, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("want object data, got %T", data)
	}
	return intFromMap(m, key)
}

func intFromMap(m map[string]any, key string) int64 {
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int64(n)
	case int64:
		return n
	default:
		return 0
	}
}

func itemsFromData(data any) []map[string]any {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	raw, ok := m["items"].([]any)
	if !ok {
		return nil
	}
	items := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		if row, ok := item.(map[string]any); ok {
			items = append(items, row)
		}
	}
	return items
}

func latestNotePublished(db *gorm.DB) (events.NotePublishedEvent, error) {
	var row dao.EventOutboxOfDB
	err := db.Where("topic = ?", events.TopicNotePublished).
		Order("id DESC").
		Take(&row).Error
	if err != nil {
		return events.NotePublishedEvent{}, err
	}
	var evt events.NotePublishedEvent
	err = json.Unmarshal(row.Payload, &evt)
	return evt, err
}

func waitUntil(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatal("timed out waiting for push feed")
}
