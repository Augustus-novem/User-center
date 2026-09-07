package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"
	"user-center/internal/domain"

	"github.com/redis/go-redis/v9"
)

const (
	noteCacheTTL         = 10 * time.Minute
	noteNegativeCacheTTL = time.Minute
)

var ErrNoteNotFound = errors.New("note cache stores not found")

type NoteCache interface {
	Get(ctx context.Context, id int64) (domain.Note, error)
	Set(ctx context.Context, note domain.Note) error
	SetNotFound(ctx context.Context, id int64) error
	Delete(ctx context.Context, id int64) error
}

type noteCacheTombstone struct {
	NotFound bool `json:"not_found"`
}

type RedisNoteCache struct {
	cmd redis.Cmdable
}

func NewRedisNoteCache(cmd redis.Cmdable) *RedisNoteCache {
	return &RedisNoteCache{cmd: cmd}
}

func (c *RedisNoteCache) Get(ctx context.Context, id int64) (domain.Note, error) {
	data, err := c.cmd.Get(ctx, c.key(id)).Bytes()
	if err != nil {
		return domain.Note{}, err
	}
	var tombstone noteCacheTombstone
	if err = json.Unmarshal(data, &tombstone); err != nil {
		return domain.Note{}, err
	}
	if tombstone.NotFound {
		return domain.Note{}, ErrNoteNotFound
	}
	var note domain.Note
	if err = json.Unmarshal(data, &note); err != nil {
		return domain.Note{}, err
	}
	return note, nil
}

func (c *RedisNoteCache) SetNotFound(ctx context.Context, id int64) error {
	data, err := json.Marshal(noteCacheTombstone{NotFound: true})
	if err != nil {
		return err
	}
	return c.cmd.Set(ctx, c.key(id), data, noteNegativeCacheTTL).Err()
}

func (c *RedisNoteCache) Set(ctx context.Context, note domain.Note) error {
	data, err := json.Marshal(note)
	if err != nil {
		return err
	}
	return c.cmd.Set(ctx, c.key(note.ID), data, noteCacheTTL).Err()
}

func (c *RedisNoteCache) Delete(ctx context.Context, id int64) error {
	return c.cmd.Del(ctx, c.key(id)).Err()
}

func (c *RedisNoteCache) key(id int64) string {
	return fmt.Sprintf("note:detail:%d", id)
}
