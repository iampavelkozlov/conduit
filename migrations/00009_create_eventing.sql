-- +goose Up
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    topic TEXT NOT NULL,
    event_key TEXT NOT NULL,
    event_type TEXT NOT NULL,
    event_version INTEGER NOT NULL CHECK (event_version > 0),
    source TEXT NOT NULL,
    aggregate_id UUID NOT NULL,
    payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    published_at TIMESTAMPTZ,
    last_error TEXT
);

CREATE INDEX idx_outbox_events_pending
    ON outbox_events (available_at, occurred_at, id)
    WHERE published_at IS NULL;

CREATE TABLE inbox_events (
    event_id UUID NOT NULL,
    consumer TEXT NOT NULL,
    locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    PRIMARY KEY (event_id, consumer)
);

-- +goose Down
DROP TABLE inbox_events;
DROP TABLE outbox_events;
