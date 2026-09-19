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
CREATE INDEX idx_outbox_events_pending ON outbox_events (available_at, occurred_at, id) WHERE published_at IS NULL;

CREATE TABLE inbox_events (
    event_id UUID NOT NULL,
    consumer TEXT NOT NULL,
    locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ,
    last_error TEXT,
    PRIMARY KEY (event_id, consumer)
);

-- +goose StatementBegin
CREATE FUNCTION emit_subscription_event() RETURNS TRIGGER AS $$
DECLARE
    event_name TEXT;
    row_value follows%ROWTYPE;
BEGIN
    IF TG_OP = 'INSERT' THEN
        event_name := 'subscription.created.v1';
        row_value := NEW;
    ELSE
        event_name := 'subscription.deleted.v1';
        row_value := OLD;
    END IF;
    INSERT INTO outbox_events (topic, event_key, event_type, event_version, source, aggregate_id, payload)
    VALUES (
        'conduit.subscriptions', row_value.follower_id::TEXT, event_name, 1,
        'subscriptions', row_value.follower_id,
        jsonb_build_object('follower_id', row_value.follower_id, 'followee_id', row_value.followee_id)
    );
    RETURN row_value;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER follows_event_outbox
AFTER INSERT OR DELETE ON follows
FOR EACH ROW EXECUTE FUNCTION emit_subscription_event();

-- +goose Down
DROP TRIGGER follows_event_outbox ON follows;
DROP FUNCTION emit_subscription_event();
DROP TABLE inbox_events;
DROP TABLE outbox_events;
