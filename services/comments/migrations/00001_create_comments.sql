-- +goose Up
CREATE TABLE comments (
    id UUID PRIMARY KEY CONSTRAINT comments_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(id), 6) >> 4) = 8),
    article_id UUID NOT NULL CONSTRAINT comments_article_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(article_id), 6) >> 4) = 8),
    author_id UUID NOT NULL CONSTRAINT comments_author_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(author_id), 6) >> 4) = 8),
    body TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_comments_article_id_created_at
    ON comments(article_id, created_at DESC, id DESC);
CREATE INDEX idx_comments_author_id ON comments(author_id);

-- +goose Down
DROP TABLE comments;
