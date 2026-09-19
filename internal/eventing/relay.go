package eventing

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/google/uuid"
)

type outbox interface {
	Claim(context.Context, int, time.Duration) ([]OutboxEvent, error)
	MarkPublished(context.Context, uuid.UUID) error
	Release(context.Context, uuid.UUID, time.Duration, error) error
}

type RelayConfig struct {
	BatchSize     int
	PollInterval  time.Duration
	LockTimeout   time.Duration
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
}

func (c RelayConfig) Validate() error {
	switch {
	case c.BatchSize <= 0:
		return errors.New("outbox batch size must be positive")
	case c.BatchSize > math.MaxInt32:
		return errors.New("outbox batch size exceeds database limit")
	case c.PollInterval <= 0:
		return errors.New("outbox poll interval must be positive")
	case c.LockTimeout <= 0:
		return errors.New("outbox lock timeout must be positive")
	case c.RetryDelay <= 0:
		return errors.New("outbox retry delay must be positive")
	case c.MaxRetryDelay < c.RetryDelay:
		return errors.New("outbox max retry delay must be at least retry delay")
	default:
		return nil
	}
}

type Relay struct {
	store     outbox
	publisher Publisher
	config    RelayConfig
	observer  Observer
	logger    *slog.Logger
}

func NewRelay(store outbox, publisher Publisher, config RelayConfig, observer Observer, logger *slog.Logger) (*Relay, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Relay{store: store, publisher: publisher, config: config, observer: observerOrNoop(observer), logger: logger}, nil
}

func (r *Relay) Run(ctx context.Context) error {
	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()
	for {
		if _, err := r.ProcessBatch(ctx); err != nil && !errors.Is(err, context.Canceled) {
			r.logger.ErrorContext(ctx, "outbox relay batch failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

func (r *Relay) ProcessBatch(ctx context.Context) (int, error) {
	started := time.Now()
	events, err := r.store.Claim(ctx, r.config.BatchSize, r.config.LockTimeout)
	if err != nil {
		r.observer.Observe("relay_claim", ResultFailure, time.Since(started))
		return 0, err
	}
	r.observer.Observe("relay_claim", ResultSuccess, time.Since(started))

	var failures []error
	published := 0
	for _, event := range events {
		started = time.Now()
		if err := r.publisher.Publish(ctx, event.Topic, event.Key, event.Envelope); err != nil {
			r.observer.Observe("relay_publish", ResultFailure, time.Since(started))
			delay := r.retryDelay(event.Attempts)
			if releaseErr := r.store.Release(ctx, event.Envelope.ID, delay, err); releaseErr != nil {
				failures = append(failures, fmt.Errorf("publish event %s: %w", event.Envelope.ID, errors.Join(err, releaseErr)))
			} else {
				failures = append(failures, fmt.Errorf("publish event %s: %w", event.Envelope.ID, err))
			}
			continue
		}
		if err := r.store.MarkPublished(ctx, event.Envelope.ID); err != nil {
			r.observer.Observe("relay_ack", ResultFailure, time.Since(started))
			failures = append(failures, err)
			continue
		}
		r.observer.Observe("relay_publish", ResultSuccess, time.Since(started))
		published++
	}
	return published, errors.Join(failures...)
}

func (r *Relay) retryDelay(attempt int) time.Duration {
	delay := r.config.RetryDelay
	for range max(attempt-1, 0) {
		if delay >= r.config.MaxRetryDelay/2 {
			return r.config.MaxRetryDelay
		}
		delay *= 2
	}
	return min(delay, r.config.MaxRetryDelay)
}
