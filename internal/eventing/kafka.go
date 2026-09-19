package eventing

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

type KafkaConfig struct {
	Brokers  []string
	ClientID string
	Username string
	Password string
	TLS      bool
}

func (c *KafkaConfig) Validate() error {
	switch {
	case len(c.Brokers) == 0:
		return errors.New("at least one kafka broker is required")
	case c.ClientID == "":
		return errors.New("kafka client ID must not be empty")
	case (c.Username == "") != (c.Password == ""):
		return errors.New("kafka username and password must be configured together")
	default:
		return nil
	}
}

type kafkaProducer interface {
	ProduceSync(context.Context, ...*kgo.Record) kgo.ProduceResults
	Flush(context.Context) error
	Close()
}

type KafkaPublisher struct {
	client   kafkaProducer
	observer Observer
}

func NewKafkaPublisher(config *KafkaConfig, observer Observer) (*KafkaPublisher, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	options := commonKafkaOptions(config)
	options = append(options,
		kgo.RequiredAcks(kgo.AllISRAcks()),
		kgo.ProducerBatchCompression(kgo.ZstdCompression()),
	)
	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create kafka producer: %w", err)
	}
	return &KafkaPublisher{client: client, observer: observerOrNoop(observer)}, nil
}

func (p *KafkaPublisher) Publish(ctx context.Context, topic, key string, envelope *Envelope) error {
	if err := envelope.Validate(); err != nil {
		return err
	}
	encoded, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("marshal event envelope: %w", err)
	}
	started := time.Now()
	err = p.client.ProduceSync(ctx, &kgo.Record{
		Topic: topic, Key: []byte(key), Value: encoded,
		Headers: []kgo.RecordHeader{{Key: "event_type", Value: []byte(envelope.Type)}},
	}).FirstErr()
	result := ResultSuccess
	if err != nil {
		result = ResultFailure
	}
	p.observer.Observe("kafka_publish", result, time.Since(started))
	if err != nil {
		return fmt.Errorf("publish kafka event: %w", err)
	}
	return nil
}

func (p *KafkaPublisher) Close(ctx context.Context) error {
	err := p.client.Flush(ctx)
	p.client.Close()
	return err
}

type inbox interface {
	Claim(context.Context, uuid.UUID, string, time.Duration) (bool, error)
	Complete(context.Context, uuid.UUID, string) error
	Release(context.Context, uuid.UUID, string, error) error
}

type KafkaConsumerConfig struct {
	KafkaConfig
	Group       string
	Topics      []string
	LockTimeout time.Duration
}

type KafkaConsumer struct {
	client   kafkaConsumerClient
	inbox    inbox
	group    string
	lock     time.Duration
	observer Observer
	logger   *slog.Logger
}

type kafkaConsumerClient interface {
	PollFetches(context.Context) kgo.Fetches
	CommitRecords(context.Context, ...*kgo.Record) error
	Close()
}

func NewKafkaConsumer(config *KafkaConsumerConfig, inbox inbox, observer Observer, logger *slog.Logger) (*KafkaConsumer, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if inbox == nil {
		return nil, errors.New("inbox store must not be nil")
	}
	options := commonKafkaOptions(&config.KafkaConfig)
	options = append(options,
		kgo.ConsumerGroup(config.Group), kgo.ConsumeTopics(config.Topics...),
		kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.Balancers(kgo.CooperativeStickyBalancer()),
	)
	client, err := kgo.NewClient(options...)
	if err != nil {
		return nil, fmt.Errorf("create kafka consumer: %w", err)
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &KafkaConsumer{
		client: client, inbox: inbox, group: config.Group, lock: config.LockTimeout,
		observer: observerOrNoop(observer), logger: logger,
	}, nil
}

func (c *KafkaConsumerConfig) Validate() error {
	if err := c.KafkaConfig.Validate(); err != nil {
		return err
	}
	switch {
	case c.Group == "":
		return errors.New("kafka consumer group must not be empty")
	case len(c.Topics) == 0:
		return errors.New("at least one kafka topic is required")
	case c.LockTimeout <= 0:
		return errors.New("inbox lock timeout must be positive")
	default:
		return nil
	}
}

func (c *KafkaConsumer) Run(ctx context.Context, handler Handler) error {
	defer c.client.Close()
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return nil
		}
		if fetchErrors := fetches.Errors(); len(fetchErrors) > 0 {
			errs := make([]error, len(fetchErrors))
			for i, fetchErr := range fetchErrors {
				errs[i] = fetchErr.Err
			}
			return fmt.Errorf("poll kafka: %w", errors.Join(errs...))
		}
		for _, record := range fetches.Records() {
			if err := c.consumeRecord(ctx, handler, record); err != nil {
				return err
			}
		}
	}
}

func (c *KafkaConsumer) consumeRecord(ctx context.Context, handler Handler, record *kgo.Record) error {
	started := time.Now()
	var envelope Envelope
	if err := json.Unmarshal(record.Value, &envelope); err != nil {
		c.observer.Observe("kafka_consume", ResultFailure, time.Since(started))
		return fmt.Errorf("decode kafka envelope: %w", err)
	}
	if err := envelope.Validate(); err != nil {
		c.observer.Observe("kafka_consume", ResultFailure, time.Since(started))
		return err
	}
	claimed, err := c.inbox.Claim(ctx, envelope.ID, c.group, c.lock)
	if err != nil {
		return err
	}
	if claimed {
		if err := handler.Handle(ctx, &envelope); err != nil {
			c.observer.Observe("kafka_consume", ResultFailure, time.Since(started))
			return errors.Join(err, c.inbox.Release(ctx, envelope.ID, c.group, err))
		}
		if err := c.inbox.Complete(ctx, envelope.ID, c.group); err != nil {
			return err
		}
	} else {
		c.logger.DebugContext(ctx, "skipping processed event", "event_id", envelope.ID.String())
	}
	if err := c.client.CommitRecords(ctx, record); err != nil {
		return fmt.Errorf("commit kafka offset: %w", err)
	}
	c.observer.Observe("kafka_consume", ResultSuccess, time.Since(started))
	return nil
}

func commonKafkaOptions(config *KafkaConfig) []kgo.Opt {
	options := []kgo.Opt{kgo.SeedBrokers(config.Brokers...), kgo.ClientID(config.ClientID)}
	if config.Username != "" {
		options = append(options, kgo.SASL(plain.Auth{User: config.Username, Pass: config.Password}.AsMechanism()))
	}
	if config.TLS {
		options = append(options, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}
	return options
}
