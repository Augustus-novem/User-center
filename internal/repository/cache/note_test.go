package cache

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"
	"time"
	"user-center/internal/domain"

	"github.com/redis/go-redis/v9"
)

type noteCmdableStub struct {
	redis.Cmdable
	getFn func(ctx context.Context, key string) *redis.StringCmd
	setFn func(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	delFn func(ctx context.Context, keys ...string) *redis.IntCmd
}

func (s *noteCmdableStub) Get(ctx context.Context, key string) *redis.StringCmd {
	return s.getFn(ctx, key)
}

func (s *noteCmdableStub) Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	return s.setFn(ctx, key, value, expiration)
}

func (s *noteCmdableStub) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	return s.delFn(ctx, keys...)
}

func TestRedisNoteCache_Get(t *testing.T) {
	t.Parallel()
	want := domain.Note{ID: 12, AuthorID: 3, Title: "cached", Images: []domain.NoteImage{{URL: "https://img"}}}
	data, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	var gotKey string
	c := NewRedisNoteCache(&noteCmdableStub{getFn: func(ctx context.Context, key string) *redis.StringCmd {
		gotKey = key
		return redis.NewStringResult(string(data), nil)
	}})

	got, err := c.Get(context.Background(), want.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if gotKey != "note:detail:12" || !reflect.DeepEqual(got, want) {
		t.Fatalf("key=%q note=%+v", gotKey, got)
	}
}

func TestRedisNoteCache_GetMiss(t *testing.T) {
	t.Parallel()
	c := NewRedisNoteCache(&noteCmdableStub{getFn: func(ctx context.Context, key string) *redis.StringCmd {
		return redis.NewStringResult("", redis.Nil)
	}})
	_, err := c.Get(context.Background(), 1)
	if !errors.Is(err, redis.Nil) {
		t.Fatalf("want redis.Nil, got %v", err)
	}
}

func TestRedisNoteCache_NegativeEntry(t *testing.T) {
	t.Parallel()
	var stored string
	var ttl time.Duration
	cmd := &noteCmdableStub{
		setFn: func(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
			stored = string(value.([]byte))
			ttl = expiration
			return redis.NewStatusResult("OK", nil)
		},
		getFn: func(ctx context.Context, key string) *redis.StringCmd {
			return redis.NewStringResult(stored, nil)
		},
	}
	c := NewRedisNoteCache(cmd)
	if err := c.SetNotFound(context.Background(), 5); err != nil {
		t.Fatalf("set not found: %v", err)
	}
	_, err := c.Get(context.Background(), 5)
	if !errors.Is(err, ErrNoteNotFound) || ttl != noteNegativeCacheTTL {
		t.Fatalf("err=%v ttl=%v", err, ttl)
	}
}

func TestRedisNoteCache_SetAndDelete(t *testing.T) {
	t.Parallel()
	var setKey string
	var ttl time.Duration
	var deleted string
	c := NewRedisNoteCache(&noteCmdableStub{
		setFn: func(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
			setKey, ttl = key, expiration
			return redis.NewStatusResult("OK", nil)
		},
		delFn: func(ctx context.Context, keys ...string) *redis.IntCmd {
			deleted = keys[0]
			return redis.NewIntResult(1, nil)
		},
	})

	if err := c.Set(context.Background(), domain.Note{ID: 7}); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := c.Delete(context.Background(), 7); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if setKey != "note:detail:7" || deleted != setKey || ttl != noteCacheTTL {
		t.Fatalf("set=%q deleted=%q ttl=%v", setKey, deleted, ttl)
	}
}
