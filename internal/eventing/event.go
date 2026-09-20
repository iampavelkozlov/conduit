package eventing

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Type string

const (
	SubscriptionCreated  Type = "subscription.created.v1"
	SubscriptionDeleted  Type = "subscription.deleted.v1"
	ArticleFavorited     Type = "article.favorited.v1"
	ArticleUnfavorited   Type = "article.unfavorited.v1"
	ArticleDeleted       Type = "article.deleted.v1"
	ProfileUpdated       Type = "profile.updated.v1"
	CurrentSchemaVersion      = 1
)

var knownTypes = map[Type]struct{}{
	SubscriptionCreated: {}, SubscriptionDeleted: {}, ArticleFavorited: {},
	ArticleUnfavorited: {}, ArticleDeleted: {}, ProfileUpdated: {},
}

type Envelope struct {
	ID            uuid.UUID       `json:"event_id"`
	Type          Type            `json:"event_type"`
	Version       int             `json:"event_version"`
	Source        string          `json:"source"`
	AggregateID   uuid.UUID       `json:"aggregate_id"`
	OccurredAt    time.Time       `json:"occurred_at"`
	CorrelationID string          `json:"correlation_id,omitempty"`
	Data          json.RawMessage `json:"data"`
}

type SubscriptionChangedData struct {
	FollowerID uuid.UUID `json:"follower_id"`
	FolloweeID uuid.UUID `json:"followee_id"`
}

type ArticleFavoriteChangedData struct {
	ArticleID uuid.UUID `json:"article_id"`
	UserID    uuid.UUID `json:"user_id"`
}

type ArticleDeletedData struct {
	ArticleID uuid.UUID `json:"article_id"`
	Slug      string    `json:"slug"`
	AuthorID  uuid.UUID `json:"author_id"`
}

type ProfileUpdatedData struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
	Bio      *string   `json:"bio"`
	Image    *string   `json:"image"`
}

func NewEnvelope(eventType Type, source string, aggregateID uuid.UUID, occurredAt time.Time, payload any) (*Envelope, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal event payload: %w", err)
	}
	envelope := Envelope{
		ID: uuid.New(), Type: eventType, Version: CurrentSchemaVersion, Source: source,
		AggregateID: aggregateID, OccurredAt: occurredAt.UTC(), Data: data,
	}
	if err := envelope.Validate(); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func (e *Envelope) Validate() error {
	switch {
	case e.ID == uuid.Nil:
		return errors.New("event ID must not be nil")
	case e.AggregateID == uuid.Nil:
		return errors.New("aggregate ID must not be nil")
	case e.Source == "":
		return errors.New("event source must not be empty")
	case e.OccurredAt.IsZero():
		return errors.New("event occurrence time must not be zero")
	case e.Version != CurrentSchemaVersion:
		return fmt.Errorf("unsupported event schema version %d", e.Version)
	case len(e.Data) == 0 || !json.Valid(e.Data):
		return errors.New("event data must be valid JSON")
	}
	if _, ok := knownTypes[e.Type]; !ok {
		return fmt.Errorf("unknown event type %q", e.Type)
	}
	return nil
}

func DecodePayload[T any](e *Envelope) (T, error) {
	var payload T
	if err := json.Unmarshal(e.Data, &payload); err != nil {
		return payload, fmt.Errorf("decode %s payload: %w", e.Type, err)
	}
	return payload, nil
}
