package repository

import (
	"context"
	"user-center/internal/repository/dao"
)

type EventOutboxMessage struct {
	ID         int64
	Topic      string
	MessageKey string
	Payload    []byte
	Attempts   int
	LastError  string
	Ctime      int64
	Utime      int64
}

type EventOutboxRepository interface {
	Add(ctx context.Context, topic string, key string, payload []byte) (EventOutboxMessage, error)
	ListPending(ctx context.Context, limit int) ([]EventOutboxMessage, error)
	MarkPublished(ctx context.Context, id int64) error
	MarkFailed(ctx context.Context, id int64, reason string) error
}

type EventOutboxRepositoryImpl struct {
	dao dao.EventOutboxDAO
}

func NewEventOutboxRepositoryImpl(dao dao.EventOutboxDAO) *EventOutboxRepositoryImpl {
	return &EventOutboxRepositoryImpl{dao: dao}
}

func (r *EventOutboxRepositoryImpl) Add(ctx context.Context, topic string, key string, payload []byte) (EventOutboxMessage, error) {
	row, err := r.dao.Insert(ctx, dao.EventOutboxOfDB{
		Topic:      topic,
		MessageKey: key,
		Payload:    payload,
	})
	if err != nil {
		return EventOutboxMessage{}, err
	}
	return toEventOutboxMessage(row), nil
}

func (r *EventOutboxRepositoryImpl) ListPending(ctx context.Context, limit int) ([]EventOutboxMessage, error) {
	rows, err := r.dao.ListPending(ctx, limit)
	if err != nil {
		return nil, err
	}
	res := make([]EventOutboxMessage, 0, len(rows))
	for _, row := range rows {
		res = append(res, toEventOutboxMessage(row))
	}
	return res, nil
}

func (r *EventOutboxRepositoryImpl) MarkPublished(ctx context.Context, id int64) error {
	return r.dao.MarkPublished(ctx, id)
}

func (r *EventOutboxRepositoryImpl) MarkFailed(ctx context.Context, id int64, reason string) error {
	return r.dao.MarkFailed(ctx, id, reason)
}

func toEventOutboxMessage(row dao.EventOutboxOfDB) EventOutboxMessage {
	return EventOutboxMessage{
		ID:         row.ID,
		Topic:      row.Topic,
		MessageKey: row.MessageKey,
		Payload:    row.Payload,
		Attempts:   row.Attempts,
		LastError:  row.LastError,
		Ctime:      row.Ctime,
		Utime:      row.Utime,
	}
}
