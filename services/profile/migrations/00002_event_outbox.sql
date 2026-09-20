-- +goose Up
CREATE TABLE outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(), topic TEXT NOT NULL, event_key TEXT NOT NULL,
    event_type TEXT NOT NULL, event_version INTEGER NOT NULL CHECK (event_version > 0),
    source TEXT NOT NULL, aggregate_id UUID NOT NULL, payload JSONB NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), available_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    locked_at TIMESTAMPTZ, attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    published_at TIMESTAMPTZ, last_error TEXT
);
CREATE INDEX idx_outbox_events_pending ON outbox_events (available_at, occurred_at, id) WHERE published_at IS NULL;
CREATE TABLE inbox_events (
    event_id UUID NOT NULL, consumer TEXT NOT NULL, locked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ, last_error TEXT, PRIMARY KEY (event_id, consumer)
);

-- +goose StatementBegin
CREATE FUNCTION emit_profile_updated_event() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO outbox_events (topic, event_key, event_type, event_version, source, aggregate_id, payload)
    VALUES ('conduit.profiles', NEW.user_id::TEXT, 'profile.updated.v1', 1, 'profile', NEW.user_id,
            jsonb_build_object('user_id', NEW.user_id, 'username', NEW.username, 'bio', NEW.bio, 'image', NEW.image));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER profile_updated_event_outbox
AFTER UPDATE ON profiles
FOR EACH ROW WHEN (OLD IS DISTINCT FROM NEW)
EXECUTE FUNCTION emit_profile_updated_event();

-- +goose Down
DROP TRIGGER profile_updated_event_outbox ON profiles;
DROP FUNCTION emit_profile_updated_event();
DROP TABLE inbox_events;
DROP TABLE outbox_events;
