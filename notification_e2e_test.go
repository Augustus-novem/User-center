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
	"user-center/internal/notification"
	"user-center/internal/repository"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/internal/worker"
	"user-center/ioc"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func TestNotification_CommunityEventsThroughKafkaE2E(t *testing.T) {
	cfg := loadE2EConfig(t)
	cfg.Kafka.ConsumerGroup = "user-center-notification-e2e-" + uuid.NewString()
	cfg.Kafka.ClientID = "user-center-notification-e2e"
	pingE2EDeps(t, cfg)

	appLogger := logger.NewNoOpLogger()
	if err := ioc.EnsureKafkaTopics(&cfg, appLogger); err != nil {
		t.Fatalf("ensure topics: %v", err)
	}
	db := ioc.InitDB(&cfg)
	repo := repository.NewNotificationRepositoryImpl(dao.NewGORMNotificationDAO(db), dao.NewGORMNoteDAO(db))
	svc := service.NewNotificationServiceImpl(repo)
	handler := notification.NewCommunityHandler(svc, appLogger)

	group := ioc.InitKafkaConsumerGroup(&cfg)
	t.Cleanup(func() { _ = group.Close() })
	consumeCtx, consumeCancel := context.WithCancel(context.Background())
	t.Cleanup(consumeCancel)
	go func() {
		_ = group.Consume(consumeCtx, []string{
			events.TopicUserFollowed,
			events.TopicNoteLiked,
			events.TopicCommentCreated,
		}, worker.NewConsumerGroupHandler(appLogger, map[string]worker.MessageHandler{
			events.TopicUserFollowed:   handler.HandleUserFollowed,
			events.TopicNoteLiked:      handler.HandleNoteLiked,
			events.TopicCommentCreated: handler.HandleCommentCreated,
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
	authorToken := signupAndLogin(t, httpServer.URL, suffix+"-author@notification-e2e.local", "Passw0rd!")
	actorToken := signupAndLogin(t, httpServer.URL, suffix+"-actor@notification-e2e.local", "Passw0rd!")
	authorID := profileID(t, httpServer.URL, authorToken)
	actorID := profileID(t, httpServer.URL, actorToken)

	follow := doJSON(t, httpServer.URL, http.MethodPost, fmt.Sprintf("/users/%d/follow", authorID), actorToken, nil)
	if follow.Code != 0 {
		t.Fatalf("follow: %+v", follow)
	}
	published := doJSON(t, httpServer.URL, http.MethodPost, "/notes", authorToken, map[string]any{
		"title": "notification e2e", "content": "Kafka notification flow",
	})
	if published.Code != 0 {
		t.Fatalf("publish: %+v", published)
	}
	noteID := intFromData(t, published.Data, "id")
	liked := doJSON(t, httpServer.URL, http.MethodPost, fmt.Sprintf("/notes/%d/like", noteID), actorToken, nil)
	if liked.Code != 0 {
		t.Fatalf("like: %+v", liked)
	}
	commented := doJSON(t, httpServer.URL, http.MethodPost, fmt.Sprintf("/notes/%d/comments", noteID), actorToken, map[string]any{"content": "notification e2e"})
	if commented.Code != 0 {
		t.Fatalf("comment: %+v", commented)
	}
	selfLike := doJSON(t, httpServer.URL, http.MethodPost, fmt.Sprintf("/notes/%d/like", noteID), authorToken, nil)
	if selfLike.Code != 0 {
		t.Fatalf("self like: %+v", selfLike)
	}

	var items []map[string]any
	waitUntil(t, 30*time.Second, func() bool {
		result := doJSON(t, httpServer.URL, http.MethodGet, "/notifications?limit=10", authorToken, nil)
		items = itemsFromData(result.Data)
		return result.Code == 0 && countNotificationTypes(items, actorID, noteID) == 3
	})
	if countNotificationActor(items, authorID) != 0 {
		t.Fatalf("self action created notification: %+v", items)
	}

	firstPage := doJSON(t, httpServer.URL, http.MethodGet, "/notifications?limit=1", authorToken, nil)
	firstItems := itemsFromData(firstPage.Data)
	cursor := stringFromData(firstPage.Data, "next_cursor")
	if firstPage.Code != 0 || len(firstItems) != 1 || cursor == "" {
		t.Fatalf("first page: %+v", firstPage)
	}
	secondPage := doJSON(t, httpServer.URL, http.MethodGet, "/notifications?limit=1&cursor="+url.QueryEscape(cursor), authorToken, nil)
	secondItems := itemsFromData(secondPage.Data)
	if secondPage.Code != 0 || len(secondItems) != 1 || intFromMap(firstItems[0], "id") == intFromMap(secondItems[0], "id") {
		t.Fatalf("unstable cursor pages: first=%+v second=%+v", firstPage, secondPage)
	}

	notificationID := intFromMap(firstItems[0], "id")
	for i := 0; i < 2; i++ {
		read := doJSON(t, httpServer.URL, http.MethodPost, fmt.Sprintf("/notifications/%d/read", notificationID), authorToken, nil)
		if read.Code != 0 {
			t.Fatalf("mark read attempt %d: %+v", i+1, read)
		}
	}
	var stored dao.NotificationOfDB
	if err := db.Where("id = ? AND receiver_id = ?", notificationID, authorID).Take(&stored).Error; err != nil || !stored.IsRead {
		t.Fatalf("stored read state=%+v err=%v", stored, err)
	}

	likeRow := findNotification(t, db, authorID, actorID, domain.NotificationTypeLike, noteID)
	replayed := events.NoteLikedEvent{
		EventID: likeRow.EventId, Type: events.TopicNoteLiked,
		NoteID: noteID, UserID: actorID, OccurredAt: likeRow.CreatedAt,
	}
	payload := mustJSON(replayed).Bytes()
	for i := 0; i < 2; i++ {
		if err := handler.HandleNoteLiked(context.Background(), &sarama.ConsumerMessage{Value: payload}); err != nil {
			t.Fatalf("replay attempt %d: %v", i+1, err)
		}
	}
	var duplicates int64
	if err := db.Model(&dao.NotificationOfDB{}).Where("event_id = ?", likeRow.EventId).Count(&duplicates).Error; err != nil || duplicates != 1 {
		t.Fatalf("event rows=%d err=%v", duplicates, err)
	}
}

func countNotificationTypes(items []map[string]any, actorID, noteID int64) int {
	want := map[string]bool{
		domain.NotificationTypeFollow:  false,
		domain.NotificationTypeLike:    false,
		domain.NotificationTypeComment: false,
	}
	for _, item := range items {
		if intFromMap(item, "actor_id") != actorID {
			continue
		}
		typeName, _ := item["type"].(string)
		if typeName == domain.NotificationTypeLike && intFromMap(item, "biz_id") != noteID {
			continue
		}
		if _, ok := want[typeName]; ok {
			want[typeName] = true
		}
	}
	count := 0
	for _, found := range want {
		if found {
			count++
		}
	}
	return count
}

func countNotificationActor(items []map[string]any, actorID int64) int {
	count := 0
	for _, item := range items {
		if intFromMap(item, "actor_id") == actorID {
			count++
		}
	}
	return count
}

func stringFromData(data any, key string) string {
	object, ok := data.(map[string]any)
	if !ok {
		return ""
	}
	value, _ := object[key].(string)
	return value
}

func findNotification(t *testing.T, db *gorm.DB, receiverID, actorID int64, notificationType string, bizID int64) dao.NotificationOfDB {
	t.Helper()
	var row dao.NotificationOfDB
	err := db.Where("receiver_id = ? AND actor_id = ? AND type = ? AND biz_id = ?", receiverID, actorID, notificationType, bizID).
		Order("id DESC").Take(&row).Error
	if err != nil {
		t.Fatalf("find notification: %v", err)
	}
	return row
}
