package eventing

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type fakeOutbox struct {
	events     []OutboxEvent
	claimErr   error
	markErr    error
	releaseErr error
	marked     []uuid.UUID
	released   []uuid.UUID
}

func (f *fakeOutbox) Claim(context.Context, int, time.Duration) ([]OutboxEvent, error) {
	return f.events, f.claimErr
}
func (f *fakeOutbox) MarkPublished(_ context.Context, id uuid.UUID) error {
	f.marked = append(f.marked, id)
	return f.markErr
}
func (f *fakeOutbox) Release(_ context.Context, id uuid.UUID, _ time.Duration, _ error) error {
	f.released = append(f.released, id)
	return f.releaseErr
}

type fakePublisher struct{ fail uuid.UUID }

func (f fakePublisher) Publish(_ context.Context, _ string, _ string, event *Envelope) error {
	if event.ID == f.fail {
		return errors.New("Kafka unavailable")
	}
	return nil
}

type observed struct{ calls int }

func (o *observed) Observe(string, string, time.Duration) { o.calls++ }

func TestRelayProcessesAndRetriesBatch(t *testing.T) {
	one, err := NewEnvelope(SubscriptionCreated, "subscriptions", uuid.New(), time.Now(), SubscriptionChangedData{})
	require.NoError(t, err)
	two, err := NewEnvelope(SubscriptionDeleted, "subscriptions", uuid.New(), time.Now(), SubscriptionChangedData{})
	require.NoError(t, err)
	store := &fakeOutbox{events: []OutboxEvent{{Topic: "topic", Envelope: one}, {Topic: "topic", Envelope: two, Attempts: 2}}}
	observer := &observed{}
	relay, err := NewRelay(store, fakePublisher{fail: two.ID}, validRelayConfig(), observer, slog.New(slog.DiscardHandler))
	require.NoError(t, err)
	published, err := relay.ProcessBatch(t.Context())
	require.Error(t, err)
	require.Equal(t, 1, published)
	require.Equal(t, []uuid.UUID{one.ID}, store.marked)
	require.Equal(t, []uuid.UUID{two.ID}, store.released)
	require.Positive(t, observer.calls)
	require.Equal(t, 2*time.Second, relay.retryDelay(2))
	require.Equal(t, 8*time.Second, relay.retryDelay(99))
}

func TestRelayConfigAndCancellation(t *testing.T) {
	mutations := []func(*RelayConfig){
		func(config *RelayConfig) { config.BatchSize = 0 },
		func(config *RelayConfig) { config.BatchSize = math.MaxInt32 + 1 },
		func(config *RelayConfig) { config.PollInterval = 0 },
		func(config *RelayConfig) { config.LockTimeout = 0 },
		func(config *RelayConfig) { config.RetryDelay = 0 },
		func(config *RelayConfig) { config.MaxRetryDelay = 0 },
	}
	for _, mutate := range mutations {
		config := validRelayConfig()
		mutate(&config)
		_, err := NewRelay(&fakeOutbox{}, fakePublisher{}, config, nil, nil)
		require.Error(t, err)
	}
	relay, err := NewRelay(&fakeOutbox{}, fakePublisher{}, validRelayConfig(), nil, nil)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	require.NoError(t, relay.Run(ctx))

	store := &fakeOutbox{claimErr: errors.New("database")}
	relay, err = NewRelay(store, fakePublisher{}, validRelayConfig(), nil, nil)
	require.NoError(t, err)
	_, err = relay.ProcessBatch(t.Context())
	require.ErrorIs(t, err, store.claimErr)

	event, newErr := NewEnvelope(ArticleDeleted, "posts", uuid.New(), time.Now(), ArticleDeletedData{})
	require.NoError(t, newErr)
	store = &fakeOutbox{events: []OutboxEvent{{Envelope: event}}, markErr: errors.New("mark")}
	relay, err = NewRelay(store, fakePublisher{}, validRelayConfig(), nil, nil)
	require.NoError(t, err)
	_, err = relay.ProcessBatch(t.Context())
	require.ErrorIs(t, err, store.markErr)
	store.markErr, store.releaseErr = nil, errors.New("release")
	relay.publisher = fakePublisher{fail: event.ID}
	_, err = relay.ProcessBatch(t.Context())
	require.ErrorIs(t, err, store.releaseErr)
}

func validRelayConfig() RelayConfig {
	return RelayConfig{BatchSize: 10, PollInterval: time.Millisecond, LockTimeout: time.Minute, RetryDelay: time.Second, MaxRetryDelay: 8 * time.Second}
}
