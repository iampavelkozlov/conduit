-- +goose Up
CREATE TABLE articles (
    id UUID PRIMARY KEY CONSTRAINT articles_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(id), 6) >> 4) = 8),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    slug TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_articles_author_id ON articles(author_id);

-- +goose Down
DROP TABLE articles;
