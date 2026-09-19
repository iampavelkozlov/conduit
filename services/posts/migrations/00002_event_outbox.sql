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
CREATE FUNCTION emit_favorite_event() RETURNS TRIGGER AS $$
DECLARE event_name TEXT; row_value article_favorites%ROWTYPE;
BEGIN
    IF TG_OP = 'INSERT' THEN event_name := 'article.favorited.v1'; row_value := NEW;
    ELSE event_name := 'article.unfavorited.v1'; row_value := OLD; END IF;
    INSERT INTO outbox_events (topic, event_key, event_type, event_version, source, aggregate_id, payload)
    VALUES ('conduit.articles', row_value.article_id::TEXT, event_name, 1, 'posts', row_value.article_id,
            jsonb_build_object('article_id', row_value.article_id, 'user_id', row_value.user_id));
    RETURN row_value;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER favorites_event_outbox AFTER INSERT OR DELETE ON article_favorites
FOR EACH ROW EXECUTE FUNCTION emit_favorite_event();

-- +goose StatementBegin
CREATE FUNCTION emit_article_deleted_event() RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO outbox_events (topic, event_key, event_type, event_version, source, aggregate_id, payload)
    VALUES ('conduit.articles', OLD.id::TEXT, 'article.deleted.v1', 1, 'posts', OLD.id,
            jsonb_build_object('article_id', OLD.id, 'slug', OLD.slug, 'author_id', OLD.author_id));
    RETURN OLD;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd
CREATE TRIGGER article_deleted_event_outbox AFTER DELETE ON articles
FOR EACH ROW EXECUTE FUNCTION emit_article_deleted_event();

-- +goose Down
DROP TRIGGER article_deleted_event_outbox ON articles;
DROP FUNCTION emit_article_deleted_event();
DROP TRIGGER favorites_event_outbox ON article_favorites;
DROP FUNCTION emit_favorite_event();
DROP TABLE inbox_events;
DROP TABLE outbox_events;
