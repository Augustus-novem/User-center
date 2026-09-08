//go:build e2e

package repository

import (
	"context"
	"testing"
	"user-center/internal/config"
	"user-center/internal/domain"
	"user-center/internal/repository/cache"
	"user-center/internal/repository/dao"
	"user-center/pkg/logger"

	"github.com/redis/go-redis/v9"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type alwaysMissLocalNoteCache struct{}

func (alwaysMissLocalNoteCache) Get(int64) (domain.Note, error) {
	return domain.Note{}, cache.ErrNoteCacheMiss
}
func (alwaysMissLocalNoteCache) Set(domain.Note)   {}
func (alwaysMissLocalNoteCache) SetNotFound(int64) {}
func (alwaysMissLocalNoteCache) Delete(int64)      {}

func BenchmarkNoteDetailLayers_e2e(b *testing.B) {
	mgr, err := config.NewManager("../../config/dev.yaml")
	if err != nil {
		b.Skipf("load config: %v", err)
	}
	cfg := mgr.App()
	db, err := gorm.Open(mysql.Open(cfg.DB.DSN))
	if err != nil {
		b.Skipf("mysql unavailable: %v", err)
	}
	if err = dao.InitTables(db); err != nil {
		b.Fatalf("migrate: %v", err)
	}
	rdb := redis.NewClient(&redis.Options{Addr: cfg.Redis.Addr, Password: cfg.Redis.Password, DB: cfg.Redis.DB})
	if err = rdb.Ping(context.Background()).Err(); err != nil {
		b.Skipf("redis unavailable: %v", err)
	}
	b.Cleanup(func() { _ = rdb.Close() })

	base := NewNoteRepositoryImpl(dao.NewGORMNoteDAO(db))
	note, err := base.Create(context.Background(), domain.Note{
		AuthorID: 1, Title: "m11 note detail benchmark", Content: "isolated benchmark row",
		Images: []domain.NoteImage{{URL: "https://example.local/m11.png", SortOrder: 0}},
	})
	if err != nil {
		b.Fatalf("create note: %v", err)
	}
	b.Cleanup(func() {
		_ = db.Where("note_id = ?", note.ID).Delete(&dao.NoteImageOfDB{}).Error
		_ = db.Where("id = ?", note.ID).Delete(&dao.NoteOfDB{}).Error
	})

	redisCache := cache.NewRedisNoteCache(rdb)
	b.Cleanup(func() { _ = redisCache.Delete(context.Background(), note.ID) })

	b.Run("mysql_only", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			if _, err := base.FindByID(context.Background(), note.ID); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("redis_miss_then_mysql", func(b *testing.B) {
		b.ReportAllocs()
		for range b.N {
			b.StopTimer()
			if err := redisCache.Delete(context.Background(), note.ID); err != nil {
				b.Fatal(err)
			}
			repo := NewCachedNoteRepository(base, redisCache, alwaysMissLocalNoteCache{}, logger.NewNoOpLogger())
			b.StartTimer()
			if _, err := repo.FindByID(context.Background(), note.ID); err != nil {
				b.Fatal(err)
			}
		}
	})

	if err = redisCache.Set(context.Background(), note); err != nil {
		b.Fatal(err)
	}
	b.Run("redis_hit", func(b *testing.B) {
		repo := NewCachedNoteRepository(base, redisCache, alwaysMissLocalNoteCache{}, logger.NewNoOpLogger())
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := repo.FindByID(context.Background(), note.ID); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("local_hit", func(b *testing.B) {
		local := cache.NewMemoryNoteLocalCache()
		local.Set(note)
		repo := NewCachedNoteRepository(base, redisCache, local, logger.NewNoOpLogger())
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			if _, err := repo.FindByID(context.Background(), note.ID); err != nil {
				b.Fatal(err)
			}
		}
	})
}
