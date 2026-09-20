package eventing

import (
	"context"
	"errors"
	"testing"
	"time"

	"conduit/internal/gen/postgres"
	"conduit/internal/service/shared"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/require"
)

type fakeOutboxQueries struct {
	rows     []postgres.ClaimOutboxEventsRow
	err      error
	affected int64
	released postgres.ReleaseOutboxEventParams
}

func (f *fakeOutboxQueries) ClaimOutboxEvents(context.Context, postgres.ClaimOutboxEventsParams) ([]postgres.ClaimOutboxEventsRow, error) {
	return f.rows, f.err
}
func (f *fakeOutboxQueries) MarkOutboxPublished(context.Context, pgtype.UUID) (int64, error) {
	return f.affected, f.err
}
func (f *fakeOutboxQueries) ReleaseOutboxEvent(_ context.Context, params postgres.ReleaseOutboxEventParams) (int64, error) {
	f.released = params
	return f.affected, f.err
}

type fakeInboxQueries struct {
	claimed  bool
	err      error
	affected int64
	released postgres.ReleaseInboxEventParams
}

func (f *fakeInboxQueries) ClaimInboxEvent(context.Context, postgres.ClaimInboxEventParams) (bool, error) {
	return f.claimed, f.err
}
func (f *fakeInboxQueries) CompleteInboxEvent(context.Context, postgres.CompleteInboxEventParams) (int64, error) {
	return f.affected, f.err
}
func (f *fakeInboxQueries) ReleaseInboxEvent(_ context.Context, params postgres.ReleaseInboxEventParams) (int64, error) {
	f.released = params
	return f.affected, f.err
}

func TestOutboxStoreLifecycle(t *testing.T) {
	eventID, aggregateID := uuid.New(), uuid.New()
	queries := &fakeOutboxQueries{affected: 1, rows: []postgres.ClaimOutboxEventsRow{{
		ID: shared.UUIDToPG(eventID), Topic: "topic", EventKey: "key", EventType: string(SubscriptionCreated),
		EventVersion: 1, Source: "subscriptions", AggregateID: shared.UUIDToPG(aggregateID),
		Payload:    []byte(`{"follower_id":"00000000-0000-0000-0000-000000000000"}`),
		OccurredAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}, Attempts: 2,
	}}}
	store := NewOutboxStore(queries)
	events, err := store.Claim(t.Context(), 10, time.Minute)
	require.NoError(t, err)
	require.Len(t, events, 1)
	require.Equal(t, eventID, events[0].Envelope.ID)
	require.Equal(t, 2, events[0].Attempts)
	require.NoError(t, store.MarkPublished(t.Context(), eventID))
	require.NoError(t, store.Release(t.Context(), eventID, time.Second, errors.New("failed")))
	require.Equal(t, "failed", queries.released.LastError)
}

func TestOutboxStoreErrors(t *testing.T) {
	wantErr := errors.New("database")
	queries := &fakeOutboxQueries{err: wantErr}
	store := NewOutboxStore(queries)
	_, err := store.Claim(t.Context(), 1, time.Second)
	require.ErrorIs(t, err, wantErr)
	require.ErrorIs(t, store.MarkPublished(t.Context(), uuid.New()), wantErr)
	require.ErrorIs(t, store.Release(t.Context(), uuid.New(), time.Second, nil), wantErr)

	queries.err, queries.affected = nil, 0
	require.ErrorContains(t, store.MarkPublished(t.Context(), uuid.New()), "affected 0")
	require.ErrorContains(t, store.Release(t.Context(), uuid.New(), time.Second, nil), "affected 0")

	queries.rows = []postgres.ClaimOutboxEventsRow{{ID: pgtype.UUID{}}}
	_, err = store.Claim(t.Context(), 1, time.Second)
	require.ErrorContains(t, err, "event ID")
	queries.rows[0].ID = shared.UUIDToPG(uuid.New())
	_, err = store.Claim(t.Context(), 1, time.Second)
	require.ErrorContains(t, err, "aggregate ID")
	queries.rows[0].AggregateID = shared.UUIDToPG(uuid.New())
	_, err = store.Claim(t.Context(), 1, time.Second)
	require.ErrorContains(t, err, "validate outbox event")
}

func TestInboxStoreLifecycleAndDeduplication(t *testing.T) {
	id := uuid.New()
	queries := &fakeInboxQueries{claimed: true, affected: 1}
	store := NewInboxStore(queries)
	claimed, err := store.Claim(t.Context(), id, "consumer", time.Minute)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, store.Complete(t.Context(), id, "consumer"))
	require.NoError(t, store.Release(t.Context(), id, "consumer", errors.New("handler")))
	require.Equal(t, "handler", queries.released.LastError)

	queries.err = pgx.ErrNoRows
	claimed, err = store.Claim(t.Context(), id, "consumer", time.Minute)
	require.NoError(t, err)
	require.False(t, claimed)
	queries.err = errors.New("database")
	_, err = store.Claim(t.Context(), id, "consumer", time.Minute)
	require.Error(t, err)
	require.Error(t, store.Complete(t.Context(), id, "consumer"))
	require.Error(t, store.Release(t.Context(), id, "consumer", nil))

	queries.err, queries.affected = nil, 0
	require.ErrorContains(t, store.Complete(t.Context(), id, "consumer"), "affected 0")
	require.True(t, interval(time.Second).Valid)
}
