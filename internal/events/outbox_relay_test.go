package events

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/repository"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type outboxRepositoryStub struct {
	listFn          func(context.Context, int) ([]repository.EventOutboxMessage, error)
	markPublishedFn func(context.Context, int64) error
	markFailedFn    func(context.Context, int64, string) error
}

func (*outboxRepositoryStub) Add(context.Context, string, string, []byte) (repository.EventOutboxMessage, error) {
	return repository.EventOutboxMessage{}, nil
}

func (s *outboxRepositoryStub) ListPending(ctx context.Context, limit int) ([]repository.EventOutboxMessage, error) {
	return s.listFn(ctx, limit)
}

func (s *outboxRepositoryStub) MarkPublished(ctx context.Context, id int64) error {
	return s.markPublishedFn(ctx, id)
}

func (s *outboxRepositoryStub) MarkFailed(ctx context.Context, id int64, reason string) error {
	return s.markFailedFn(ctx, id, reason)
}

type syncProducerStub struct {
	sendFn func(*sarama.ProducerMessage) (int32, int64, error)
}

func (s *syncProducerStub) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	return s.sendFn(msg)
}
func (*syncProducerStub) SendMessages([]*sarama.ProducerMessage) error { return nil }
func (*syncProducerStub) Close() error                                 { return nil }
func (*syncProducerStub) TxnStatus() sarama.ProducerTxnStatusFlag      { return 0 }
func (*syncProducerStub) IsTransactional() bool                        { return false }
func (*syncProducerStub) BeginTxn() error                              { return nil }
func (*syncProducerStub) CommitTxn() error                             { return nil }
func (*syncProducerStub) AbortTxn() error                              { return nil }
func (*syncProducerStub) AddOffsetsToTxn(map[string][]*sarama.PartitionOffsetMetadata, string) error {
	return nil
}
func (*syncProducerStub) AddMessageToTxn(*sarama.ConsumerMessage, string, *string) error { return nil }

func TestOutboxRelay_BacklogRecoversAfterKafkaFailure(t *testing.T) {
	t.Parallel()
	message := repository.EventOutboxMessage{ID: 7, Topic: TopicUserFollowed, MessageKey: "2", Payload: []byte(`{"event_id":"evt"}`)}
	published := false
	failed := 0
	repo := &outboxRepositoryStub{
		listFn: func(context.Context, int) ([]repository.EventOutboxMessage, error) {
			if published {
				return nil, nil
			}
			message.Attempts = failed
			return []repository.EventOutboxMessage{message}, nil
		},
		markPublishedFn: func(_ context.Context, id int64) error {
			if id != message.ID {
				t.Fatalf("published id=%d", id)
			}
			published = true
			return nil
		},
		markFailedFn: func(_ context.Context, id int64, reason string) error {
			if id != message.ID || reason != "kafka unavailable" {
				t.Fatalf("failed id=%d reason=%q", id, reason)
			}
			failed++
			return nil
		},
	}
	sends := 0
	producer := &syncProducerStub{sendFn: func(msg *sarama.ProducerMessage) (int32, int64, error) {
		sends++
		if sends == 1 {
			return 0, 0, errors.New("kafka unavailable")
		}
		if msg.Topic != message.Topic {
			t.Fatalf("topic=%q", msg.Topic)
		}
		return 0, 1, nil
	}}
	relay := NewOutboxRelay(repo, producer, logger.NewNoOpLogger())

	if err := relay.DispatchBatch(context.Background()); err != nil {
		t.Fatalf("failed dispatch should preserve backlog: %v", err)
	}
	if failed != 1 || published {
		t.Fatalf("failed=%d published=%v", failed, published)
	}
	if err := relay.DispatchBatch(context.Background()); err != nil {
		t.Fatalf("recovery dispatch: %v", err)
	}
	if sends != 2 || !published {
		t.Fatalf("sends=%d published=%v", sends, published)
	}
}

func TestOutboxRelay_AcknowledgedMessageCanBeRedelivered(t *testing.T) {
	t.Parallel()
	message := repository.EventOutboxMessage{ID: 9, Topic: TopicNoteLiked, MessageKey: "1", Payload: []byte(`{}`)}
	markCalls := 0
	repo := &outboxRepositoryStub{
		listFn: func(context.Context, int) ([]repository.EventOutboxMessage, error) {
			return []repository.EventOutboxMessage{message}, nil
		},
		markPublishedFn: func(context.Context, int64) error {
			markCalls++
			if markCalls == 1 {
				return errors.New("mysql unavailable after kafka ack")
			}
			return nil
		},
		markFailedFn: func(context.Context, int64, string) error { return nil },
	}
	sends := 0
	producer := &syncProducerStub{sendFn: func(*sarama.ProducerMessage) (int32, int64, error) {
		sends++
		return 0, int64(sends), nil
	}}
	relay := NewOutboxRelay(repo, producer, logger.NewNoOpLogger())

	if err := relay.DispatchBatch(context.Background()); err == nil {
		t.Fatal("expected mark-published failure")
	}
	if err := relay.DispatchBatch(context.Background()); err != nil {
		t.Fatalf("second dispatch: %v", err)
	}
	if sends != 2 {
		t.Fatalf("at-least-once window should redeliver, sends=%d", sends)
	}
}
