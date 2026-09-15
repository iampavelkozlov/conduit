-- +goose Up
CREATE TABLE article_favorites (
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (article_id, user_id)
);

CREATE INDEX idx_article_favorites_user_id ON article_favorites(user_id);

-- +goose Down
DROP TABLE article_favorites;
