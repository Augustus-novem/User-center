package worker

import (
	"context"
	"errors"
	"testing"
	"user-center/internal/events"
	"user-center/pkg/logger"

	"github.com/IBM/sarama"
)

type consumerSessionStub struct {
	ctx    context.Context
	marked []*sarama.ConsumerMessage
}

func (*consumerSessionStub) Claims() map[string][]int32 { return nil }
func (*consumerSessionStub) MemberID() string           { return "member" }
func (*consumerSessionStub) GenerationID() int32        { return 1 }
func (*consumerSessionStub) MarkOffset(string, int32, int64, string) {
}
func (*consumerSessionStub) Commit() {}
func (*consumerSessionStub) ResetOffset(string, int32, int64, string) {
}
func (s *consumerSessionStub) MarkMessage(msg *sarama.ConsumerMessage, _ string) {
	s.marked = append(s.marked, msg)
}
func (s *consumerSessionStub) Context() context.Context { return s.ctx }

type consumerClaimStub struct {
	messages <-chan *sarama.ConsumerMessage
}

type deadLetterPublisherStub struct {
	message *sarama.ProducerMessage
	err     error
}

func (s *deadLetterPublisherStub) SendMessage(msg *sarama.ProducerMessage) (int32, int64, error) {
	s.message = msg
	return 0, 0, s.err
}

func (*consumerClaimStub) Topic() string                              { return events.TopicNotePublished }
func (*consumerClaimStub) Partition() int32                           { return 0 }
func (*consumerClaimStub) InitialOffset() int64                       { return 10 }
func (*consumerClaimStub) HighWaterMarkOffset() int64                 { return 11 }
func (s *consumerClaimStub) Messages() <-chan *sarama.ConsumerMessage { return s.messages }

func TestConsumerGroupHandler_RedeliversAfterSessionRestart(t *testing.T) {
	t.Parallel()
	msg := &sarama.ConsumerMessage{Topic: events.TopicNotePublished, Partition: 0, Offset: 10}
	attempts := 0
	handler := NewConsumerGroupHandler(logger.NewNoOpLogger(), map[string]MessageHandler{
		events.TopicNotePublished: func(context.Context, *sarama.ConsumerMessage) error {
			attempts++
			if attempts == 1 {
				return errors.New("feed inbox unavailable")
			}
			return nil
		},
	})

	firstSession := &consumerSessionStub{ctx: context.Background()}
	if err := handler.ConsumeClaim(firstSession, claimWithMessages(msg)); err == nil {
		t.Fatal("first session should fail")
	}
	if len(firstSession.marked) != 0 {
		t.Fatalf("failed message must not be marked, got %d", len(firstSession.marked))
	}

	secondSession := &consumerSessionStub{ctx: context.Background()}
	if err := handler.ConsumeClaim(secondSession, claimWithMessages(msg)); err != nil {
		t.Fatalf("restarted session: %v", err)
	}
	if attempts != 2 || len(secondSession.marked) != 1 || secondSession.marked[0].Offset != msg.Offset {
		t.Fatalf("attempts=%d marked=%+v", attempts, secondSession.marked)
	}
}

func TestConsumerGroupHandler_PermanentErrorRequiresSuccessfulDLQPublish(t *testing.T) {
	t.Parallel()
	msg := &sarama.ConsumerMessage{
		Topic: events.TopicNotePublished, Partition: 2, Offset: 17,
		Key: []byte("note-1"), Value: []byte("not-json"),
	}

	t.Run("successful publish marks poison message", func(t *testing.T) {
		producer := &deadLetterPublisherStub{}
		handler := NewConsumerGroupHandlerWithDLQ(logger.NewNoOpLogger(), producer, map[string]MessageHandler{
			events.TopicNotePublished: func(context.Context, *sarama.ConsumerMessage) error {
				return Permanent(errors.New("invalid json"))
			},
		})
		session := &consumerSessionStub{ctx: context.Background()}
		if err := handler.ConsumeClaim(session, claimWithMessages(msg)); err != nil {
			t.Fatalf("consume: %v", err)
		}
		if len(session.marked) != 1 {
			t.Fatalf("poison message should be marked after DLQ ack, marked=%d", len(session.marked))
		}
		if producer.message == nil || producer.message.Topic != events.TopicNotePublished+DeadLetterSuffix {
			t.Fatalf("unexpected DLQ message: %+v", producer.message)
		}
	})

	t.Run("failed publish leaves poison message uncommitted", func(t *testing.T) {
		producer := &deadLetterPublisherStub{err: errors.New("kafka unavailable")}
		handler := NewConsumerGroupHandlerWithDLQ(logger.NewNoOpLogger(), producer, map[string]MessageHandler{
			events.TopicNotePublished: func(context.Context, *sarama.ConsumerMessage) error {
				return Permanent(errors.New("invalid json"))
			},
		})
		session := &consumerSessionStub{ctx: context.Background()}
		if err := handler.ConsumeClaim(session, claimWithMessages(msg)); err == nil {
			t.Fatal("DLQ publish failure must fail the claim")
		}
		if len(session.marked) != 0 {
			t.Fatalf("failed DLQ publish must not mark, marked=%d", len(session.marked))
		}
	})
}

func TestConsumerGroupHandler_BusyIsTransientAndUncommitted(t *testing.T) {
	t.Parallel()
	msg := &sarama.ConsumerMessage{Topic: events.TopicNotePublished, Partition: 0, Offset: 18}
	producer := &deadLetterPublisherStub{}
	handler := NewConsumerGroupHandlerWithDLQ(logger.NewNoOpLogger(), producer, map[string]MessageHandler{
		events.TopicNotePublished: func(context.Context, *sarama.ConsumerMessage) error {
			return ErrMessageInFlight
		},
	})
	session := &consumerSessionStub{ctx: context.Background()}
	err := handler.ConsumeClaim(session, claimWithMessages(msg))
	if !errors.Is(err, ErrMessageInFlight) {
		t.Fatalf("want ErrMessageInFlight, got %v", err)
	}
	if len(session.marked) != 0 || producer.message != nil {
		t.Fatalf("BUSY must neither mark nor DLQ: marked=%d dlq=%+v", len(session.marked), producer.message)
	}
}

func claimWithMessages(messages ...*sarama.ConsumerMessage) *consumerClaimStub {
	ch := make(chan *sarama.ConsumerMessage, len(messages))
	for _, msg := range messages {
		ch <- msg
	}
	close(ch)
	return &consumerClaimStub{messages: ch}
}
