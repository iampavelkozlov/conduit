package eventing

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestEnvelopeRoundTrip(t *testing.T) {
	followerID, followeeID := uuid.New(), uuid.New()
	event, err := NewEnvelope(SubscriptionCreated, "subscriptions", followerID, time.Now(), SubscriptionChangedData{
		FollowerID: followerID, FolloweeID: followeeID,
	})
	require.NoError(t, err)
	require.NoError(t, event.Validate())
	payload, err := DecodePayload[SubscriptionChangedData](event)
	require.NoError(t, err)
	require.Equal(t, followeeID, payload.FolloweeID)

	encoded, err := json.Marshal(event)
	require.NoError(t, err)
	var decoded Envelope
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	require.Equal(t, *event, decoded)
}

func TestEnvelopeValidation(t *testing.T) {
	valid, err := NewEnvelope(ProfileUpdated, "profile", uuid.New(), time.Now(), ProfileUpdatedData{})
	require.NoError(t, err)
	tests := []struct {
		name   string
		mutate func(*Envelope)
	}{
		{name: "event ID", mutate: func(event *Envelope) { event.ID = uuid.Nil }},
		{name: "aggregate ID", mutate: func(event *Envelope) { event.AggregateID = uuid.Nil }},
		{name: "source", mutate: func(event *Envelope) { event.Source = "" }},
		{name: "time", mutate: func(event *Envelope) { event.OccurredAt = time.Time{} }},
		{name: "version", mutate: func(event *Envelope) { event.Version++ }},
		{name: "payload", mutate: func(event *Envelope) { event.Data = []byte("{") }},
		{name: "type", mutate: func(event *Envelope) { event.Type = "unknown.v1" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := *valid
			tt.mutate(&event)
			require.Error(t, event.Validate())
		})
	}

	_, err = NewEnvelope(ProfileUpdated, "profile", uuid.New(), time.Now(), func() {})
	require.ErrorContains(t, err, "marshal event payload")
	_, err = NewEnvelope(ProfileUpdated, "profile", uuid.Nil, time.Now(), ProfileUpdatedData{})
	require.ErrorContains(t, err, "aggregate ID")
	valid.Data = []byte("not-json")
	_, err = DecodePayload[ProfileUpdatedData](valid)
	require.Error(t, err)
}
