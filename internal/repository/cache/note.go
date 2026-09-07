package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"user-center/internal/domain"

	"github.com/redis/go-redis/v9"
)

const noteCacheTTL = 10 * time.Minute

type NoteCache interface {
	Get(ctx context.Context, id int64) (domain.Note, error)
	Set(ctx context.Context, note domain.Note) error
	Delete(ctx context.Context, id int64) error
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
	var note domain.Note
	if err = json.Unmarshal(data, &note); err != nil {
		return domain.Note{}, err
	}
	return note, nil
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
