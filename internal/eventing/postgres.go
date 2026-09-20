package eventing

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"conduit/internal/gen/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type outboxQueries interface {
	ClaimOutboxEvents(context.Context, postgres.ClaimOutboxEventsParams) ([]postgres.ClaimOutboxEventsRow, error)
	MarkOutboxPublished(context.Context, pgtype.UUID) (int64, error)
	ReleaseOutboxEvent(context.Context, postgres.ReleaseOutboxEventParams) (int64, error)
}

type inboxQueries interface {
	ClaimInboxEvent(context.Context, postgres.ClaimInboxEventParams) (bool, error)
	CompleteInboxEvent(context.Context, postgres.CompleteInboxEventParams) (int64, error)
	ReleaseInboxEvent(context.Context, postgres.ReleaseInboxEventParams) (int64, error)
}

type OutboxEvent struct {
	Topic    string
	Key      string
	Envelope *Envelope
	Attempts int
}

type OutboxStore struct {
	queries outboxQueries
}

func NewOutboxStore(queries outboxQueries) *OutboxStore {
	return &OutboxStore{queries: queries}
}

func (s *OutboxStore) Claim(ctx context.Context, batchSize int, lockTimeout time.Duration) ([]OutboxEvent, error) {
	if batchSize <= 0 || batchSize > math.MaxInt32 {
		return nil, errors.New("outbox batch size is outside database range")
	}
	rows, err := s.queries.ClaimOutboxEvents(ctx, postgres.ClaimOutboxEventsParams{
		BatchSize: int32(batchSize), LockTimeout: interval(lockTimeout),
	})
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	events := make([]OutboxEvent, len(rows))
	for i := range rows {
		row := &rows[i]
		id, err := uuidFromPG(row.ID)
		if err != nil {
			return nil, fmt.Errorf("parse outbox event ID: %w", err)
		}
		aggregateID, err := uuidFromPG(row.AggregateID)
		if err != nil {
			return nil, fmt.Errorf("parse outbox aggregate ID: %w", err)
		}
		envelope := Envelope{
			ID: id, Type: Type(row.EventType), Version: int(row.EventVersion), Source: row.Source,
			AggregateID: aggregateID, OccurredAt: row.OccurredAt.Time, Data: row.Payload,
		}
		if err := envelope.Validate(); err != nil {
			return nil, fmt.Errorf("validate outbox event %s: %w", id, err)
		}
		events[i] = OutboxEvent{Topic: row.Topic, Key: row.EventKey, Envelope: &envelope, Attempts: int(row.Attempts)}
	}
	return events, nil
}

func (s *OutboxStore) MarkPublished(ctx context.Context, id uuid.UUID) error {
	rows, err := s.queries.MarkOutboxPublished(ctx, uuidToPG(id))
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("mark outbox event published: affected %d rows", rows)
	}
	return nil
}

func (s *OutboxStore) Release(ctx context.Context, id uuid.UUID, retryDelay time.Duration, cause error) error {
	message := "unknown publish failure"
	if cause != nil {
		message = cause.Error()
	}
	rows, err := s.queries.ReleaseOutboxEvent(ctx, postgres.ReleaseOutboxEventParams{
		ID: uuidToPG(id), RetryDelay: interval(retryDelay), LastError: message,
	})
	if err != nil {
		return fmt.Errorf("release outbox event: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("release outbox event: affected %d rows", rows)
	}
	return nil
}

type InboxStore struct {
	queries inboxQueries
}

func NewInboxStore(queries inboxQueries) *InboxStore {
	return &InboxStore{queries: queries}
}

func (s *InboxStore) Claim(ctx context.Context, id uuid.UUID, consumer string, lockTimeout time.Duration) (bool, error) {
	claimed, err := s.queries.ClaimInboxEvent(ctx, postgres.ClaimInboxEventParams{
		EventID: uuidToPG(id), Consumer: consumer, LockTimeout: interval(lockTimeout),
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim inbox event: %w", err)
	}
	return claimed, nil
}

func (s *InboxStore) Complete(ctx context.Context, id uuid.UUID, consumer string) error {
	rows, err := s.queries.CompleteInboxEvent(ctx, postgres.CompleteInboxEventParams{EventID: uuidToPG(id), Consumer: consumer})
	if err != nil {
		return fmt.Errorf("complete inbox event: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("complete inbox event: affected %d rows", rows)
	}
	return nil
}

func (s *InboxStore) Release(ctx context.Context, id uuid.UUID, consumer string, cause error) error {
	message := "unknown handler failure"
	if cause != nil {
		message = cause.Error()
	}
	_, err := s.queries.ReleaseInboxEvent(ctx, postgres.ReleaseInboxEventParams{
		EventID: uuidToPG(id), Consumer: consumer, LastError: message,
	})
	if err != nil {
		return fmt.Errorf("release inbox event: %w", err)
	}
	return nil
}

func interval(duration time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: duration.Microseconds(), Valid: true}
}

func uuidToPG(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func uuidFromPG(value pgtype.UUID) (uuid.UUID, error) {
	if !value.Valid {
		return uuid.Nil, errors.New("invalid UUID")
	}
	return uuid.UUID(value.Bytes), nil
}
