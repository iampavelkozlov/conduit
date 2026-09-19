package eventing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/twmb/franz-go/pkg/kgo"
)

type fakeProducer struct {
	record  *kgo.Record
	err     error
	flushed bool
	closed  bool
}

func (f *fakeProducer) ProduceSync(_ context.Context, records ...*kgo.Record) kgo.ProduceResults {
	f.record = records[0]
	return kgo.ProduceResults{{Record: records[0], Err: f.err}}
}
func (f *fakeProducer) Flush(context.Context) error { f.flushed = true; return f.err }
func (f *fakeProducer) Close()                      { f.closed = true }

type fakeInbox struct {
	claimed     bool
	completed   int
	released    int
	claimErr    error
	completeErr error
	releaseErr  error
}

func (f *fakeInbox) Claim(context.Context, uuid.UUID, string, time.Duration) (bool, error) {
	return f.claimed, f.claimErr
}
func (f *fakeInbox) Complete(context.Context, uuid.UUID, string) error {
	f.completed++
	return f.completeErr
}
func (f *fakeInbox) Release(context.Context, uuid.UUID, string, error) error {
	f.released++
	return f.releaseErr
}

type fakeConsumerClient struct {
	commits   int
	commitErr error
	fetches   []kgo.Fetches
	index     int
	closed    bool
}

func (f *fakeConsumerClient) PollFetches(context.Context) kgo.Fetches {
	if f.index >= len(f.fetches) {
		return kgo.NewErrFetch(errors.New("poll"))
	}
	fetch := f.fetches[f.index]
	f.index++
	return fetch
}
func (f *fakeConsumerClient) CommitRecords(context.Context, ...*kgo.Record) error {
	f.commits++
	return f.commitErr
}
func (f *fakeConsumerClient) Close() { f.closed = true }

func TestKafkaPublisher(t *testing.T) {
	event, err := NewEnvelope(ArticleDeleted, "posts", uuid.New(), time.Now(), ArticleDeletedData{})
	require.NoError(t, err)
	producer := &fakeProducer{}
	publisher := &KafkaPublisher{client: producer, observer: noopObserver{}}
	require.NoError(t, publisher.Publish(t.Context(), "conduit.articles", "key", event))
	require.Equal(t, "conduit.articles", producer.record.Topic)
	require.Equal(t, []byte("key"), producer.record.Key)
	require.Equal(t, "event_type", producer.record.Headers[0].Key)
	require.NoError(t, publisher.Close(t.Context()))
	require.True(t, producer.flushed)
	require.True(t, producer.closed)

	producer.err = errors.New("unavailable")
	require.Error(t, publisher.Publish(t.Context(), "topic", "key", event))
	require.Error(t, publisher.Close(t.Context()))
	event.ID = uuid.Nil
	require.Error(t, publisher.Publish(t.Context(), "topic", "key", event))
}

func TestKafkaConfigValidation(t *testing.T) {
	valid := KafkaConfig{Brokers: []string{"localhost:9092"}, ClientID: "client"}
	require.NoError(t, valid.Validate())
	_, err := NewKafkaPublisher(&KafkaConfig{}, nil)
	require.Error(t, err)
	valid.Username = "user"
	require.Error(t, valid.Validate())
	valid.Password = "password"
	require.NoError(t, valid.Validate())
	require.NotEmpty(t, commonKafkaOptions(&valid))
	valid.TLS = true
	require.NotEmpty(t, commonKafkaOptions(&valid))
	publisher, err := NewKafkaPublisher(&valid, nil)
	require.NoError(t, err)
	require.NoError(t, publisher.Close(t.Context()))

	consumerConfig := KafkaConsumerConfig{KafkaConfig: valid, Group: "group", Topics: []string{"topic"}, LockTimeout: time.Minute}
	consumer, err := NewKafkaConsumer(&consumerConfig, &fakeInbox{}, nil, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, consumer.Run(ctx, HandlerFunc(func(context.Context, *Envelope) error { return nil })))
	consumerConfig.Group = ""
	_, err = NewKafkaConsumer(&consumerConfig, &fakeInbox{}, nil, nil)
	require.Error(t, err)
	consumerConfig.Group, consumerConfig.Topics = "group", nil
	_, err = NewKafkaConsumer(&consumerConfig, &fakeInbox{}, nil, nil)
	require.Error(t, err)
	consumerConfig.Topics, consumerConfig.LockTimeout = []string{"topic"}, 0
	_, err = NewKafkaConsumer(&consumerConfig, &fakeInbox{}, nil, nil)
	require.Error(t, err)
	consumerConfig.LockTimeout = time.Minute
	_, err = NewKafkaConsumer(&consumerConfig, nil, nil, nil)
	require.Error(t, err)
}

func TestKafkaConsumerProcessesAndDeduplicates(t *testing.T) {
	event, err := NewEnvelope(ProfileUpdated, "profile", uuid.New(), time.Now(), ProfileUpdatedData{})
	require.NoError(t, err)
	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	record := &kgo.Record{Value: encoded}

	inbox := &fakeInbox{claimed: true}
	client := &fakeConsumerClient{}
	consumer := &KafkaConsumer{client: client, inbox: inbox, group: "group", lock: time.Minute, observer: noopObserver{}, logger: slog.New(slog.DiscardHandler)}
	handled := 0
	require.NoError(t, consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { handled++; return nil }), record))
	require.Equal(t, 1, handled)
	require.Equal(t, 1, inbox.completed)
	require.Equal(t, 1, client.commits)

	inbox.claimed = false
	require.NoError(t, consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { t.Fatal("duplicate handled"); return nil }), record))
	require.Equal(t, 2, client.commits)

	inbox.claimed = true
	handlerErr := errors.New("handler")
	err = consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { return handlerErr }), record)
	require.ErrorIs(t, err, handlerErr)
	require.Equal(t, 1, inbox.released)

	require.Error(t, consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { return nil }), &kgo.Record{Value: []byte("bad")}))
	inbox.claimErr = errors.New("inbox")
	require.ErrorIs(t, consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { return nil }), record), inbox.claimErr)
	inbox.claimErr, inbox.completeErr = nil, errors.New("complete")
	require.ErrorIs(t, consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { return nil }), record), inbox.completeErr)
	inbox.completeErr, client.commitErr = nil, errors.New("commit")
	require.ErrorIs(t, consumer.consumeRecord(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { return nil }), record), client.commitErr)
}

func TestKafkaConsumerRunProcessesFetch(t *testing.T) {
	event, err := NewEnvelope(ArticleDeleted, "posts", uuid.New(), time.Now(), ArticleDeletedData{})
	require.NoError(t, err)
	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	record := &kgo.Record{Value: encoded}
	fetch := kgo.Fetches{{Topics: []kgo.FetchTopic{{
		Topic: "topic", Partitions: []kgo.FetchPartition{{Records: []*kgo.Record{record}}},
	}}}}
	client := &fakeConsumerClient{fetches: []kgo.Fetches{fetch}}
	consumer := &KafkaConsumer{client: client, inbox: &fakeInbox{claimed: true}, group: "group", lock: time.Minute, observer: noopObserver{}, logger: slog.New(slog.DiscardHandler)}
	handled := 0
	err = consumer.Run(t.Context(), HandlerFunc(func(context.Context, *Envelope) error { handled++; return nil }))
	require.ErrorContains(t, err, "poll kafka")
	require.Equal(t, 1, handled)
	require.True(t, client.closed)
}
