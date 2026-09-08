//go:build e2e

package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"
	"user-center/internal/domain"
	"user-center/internal/events"
	"user-center/internal/repository"
	"user-center/internal/repository/dao"
	"user-center/internal/service"
	"user-center/ioc"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func BenchmarkOutboxKafkaLatency_e2e(b *testing.B) {
	cfg := loadE2EConfig(b)
	pingE2EDeps(b, cfg)
	topic := "m11-outbox-" + uuid.NewString()
	adminConfig := sarama.NewConfig()
	adminConfig.ClientID = "user-center-m11-benchmark-admin"
	admin, err := sarama.NewClusterAdmin(cfg.Kafka.Brokers, adminConfig)
	if err != nil {
		b.Skipf("kafka admin unavailable: %v", err)
	}
	if err = admin.CreateTopic(topic, &sarama.TopicDetail{NumPartitions: 1, ReplicationFactor: 1}, false); err != nil {
		_ = admin.Close()
		b.Fatalf("create topic: %v", err)
	}
	b.Cleanup(func() {
		_ = admin.DeleteTopic(topic)
		_ = admin.Close()
	})

	consumer, err := sarama.NewConsumer(cfg.Kafka.Brokers, sarama.NewConfig())
	if err != nil {
		b.Fatalf("consumer: %v", err)
	}
	b.Cleanup(func() { _ = consumer.Close() })
	partition, err := consumer.ConsumePartition(topic, 0, sarama.OffsetNewest)
	if err != nil {
		b.Fatalf("partition consumer: %v", err)
	}
	b.Cleanup(func() { _ = partition.Close() })

	producerConfig := sarama.NewConfig()
	producerConfig.ClientID = "user-center-m11-benchmark-producer"
	producerConfig.Producer.Return.Successes = true
	producerConfig.Producer.RequiredAcks = sarama.WaitForAll
	producer, err := sarama.NewSyncProducer(cfg.Kafka.Brokers, producerConfig)
	if err != nil {
		b.Fatalf("producer: %v", err)
	}
	db := isolatedOutboxBenchmarkDB(b, cfg.DB.DSN)
	outboxRepo := repository.NewEventOutboxRepositoryImpl(dao.NewGORMEventOutboxDAO(db))
	relay := events.NewOutboxRelay(outboxRepo, producer, logger.NewNoOpLogger())
	b.Cleanup(func() {
		_ = relay.Close()
		_ = db.Where("topic = ?", topic).Delete(&dao.EventOutboxOfDB{}).Error
	})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		payload := []byte(fmt.Sprintf(`{"event_id":"%s","sequence":%d}`, uuid.NewString(), i))
		if _, err = outboxRepo.Add(context.Background(), topic, "m11", payload); err != nil {
			b.Fatal(err)
		}
		if err = relay.DispatchBatch(context.Background()); err != nil {
			b.Fatal(err)
		}
		select {
		case msg := <-partition.Messages():
			if msg == nil || msg.Topic != topic {
				b.Fatalf("unexpected message: %+v", msg)
			}
		case err = <-partition.Errors():
			b.Fatal(err)
		case <-time.After(5 * time.Second):
			b.Fatal("timed out waiting for Kafka message")
		}
	}
}

func isolatedOutboxBenchmarkDB(b *testing.B, dsn string) *gorm.DB {
	b.Helper()
	dsnConfig, err := mysqlDriver.ParseDSN(dsn)
	if err != nil {
		b.Fatalf("parse MySQL DSN: %v", err)
	}
	databaseName := "user_center_m11_" + uuid.NewString()
	databaseName = replaceHyphens(databaseName)
	adminConfig := *dsnConfig
	adminConfig.DBName = ""
	adminDB, err := gorm.Open(mysql.Open(adminConfig.FormatDSN()))
	if err != nil {
		b.Fatalf("open MySQL admin connection: %v", err)
	}
	if err = adminDB.Exec("CREATE DATABASE `" + databaseName + "`").Error; err != nil {
		b.Fatalf("create isolated database: %v", err)
	}
	dsnConfig.DBName = databaseName
	db, err := gorm.Open(mysql.Open(dsnConfig.FormatDSN()))
	if err != nil {
		b.Fatalf("open isolated database: %v", err)
	}
	if err = db.AutoMigrate(&dao.EventOutboxOfDB{}); err != nil {
		b.Fatalf("migrate isolated outbox: %v", err)
	}
	b.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		_ = adminDB.Exec("DROP DATABASE `" + databaseName + "`").Error
		if sqlDB, dbErr := adminDB.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func replaceHyphens(value string) string {
	result := []byte(value)
	for i := range result {
		if result[i] == '-' {
			result[i] = '_'
		}
	}
	return string(result)
}

func BenchmarkSearchPaths_e2e(b *testing.B) {
	cfg := loadE2EConfig(b)
	pingE2EDeps(b, cfg)
	cfg.Search.Enabled = true
	cfg.Search.Index = "community_notes_m11_" + uuid.NewString()
	index := ioc.InitNoteSearchIndex(&cfg)
	if err := index.EnsureIndex(context.Background()); err != nil {
		b.Skipf("Elasticsearch unavailable: %v", err)
	}
	b.Cleanup(func() {
		req, _ := http.NewRequest(http.MethodDelete, cfg.Search.Address+"/"+url.PathEscape(cfg.Search.Index), nil)
		if req != nil {
			if resp, err := http.DefaultClient.Do(req); err == nil {
				_ = resp.Body.Close()
			}
		}
	})

	db := ioc.InitDB(&cfg)
	notes := repository.NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	keyword := "m11-search-" + uuid.NewString()
	note, err := notes.Create(context.Background(), domain.Note{
		AuthorID: 1, Title: keyword, Content: "search benchmark", Status: domain.NoteStatusPublished,
	})
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { _ = db.Where("id = ?", note.ID).Delete(&dao.NoteOfDB{}).Error })
	if err = index.Index(context.Background(), note); err != nil {
		b.Fatal(err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		found, searchErr := index.Search(context.Background(), keyword, 10)
		if searchErr == nil && len(found) == 1 {
			break
		}
		if time.Now().After(deadline) {
			b.Fatalf("index not visible: %v", searchErr)
		}
		time.Sleep(100 * time.Millisecond)
	}

	esService := service.NewSearchServiceImpl(true, index, notes,
		cfg.Search.RequestTimeout, cfg.Search.DBFallbackTimeout, cfg.Search.FallbackWindow, cfg.Search.MaxLimit,
		logger.NewNoOpLogger())
	fallbackService := service.NewSearchServiceImpl(false, index, notes,
		cfg.Search.RequestTimeout, cfg.Search.DBFallbackTimeout, cfg.Search.FallbackWindow, cfg.Search.MaxLimit,
		logger.NewNoOpLogger())

	b.Run("elasticsearch_warm", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			result, err := esService.SearchNotes(context.Background(), keyword, 10)
			if err != nil || result.Degraded || len(result.Items) != 1 {
				b.Fatalf("result=%+v err=%v", result, err)
			}
		}
	})
	b.Run("mysql_bounded_fallback", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			result, err := fallbackService.SearchNotes(context.Background(), keyword, 10)
			if err != nil || !result.Degraded || len(result.Items) != 1 {
				b.Fatalf("result=%+v err=%v", result, err)
			}
		}
	})
}
