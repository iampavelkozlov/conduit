-- +goose Up
CREATE TABLE follows (
    follower_id UUID NOT NULL CONSTRAINT follows_follower_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(follower_id), 6) >> 4) = 8),
    followee_id UUID NOT NULL CONSTRAINT follows_followee_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(followee_id), 6) >> 4) = 8),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (follower_id, followee_id),
    CHECK (follower_id <> followee_id)
);

CREATE INDEX idx_follows_followee_id ON follows(followee_id);

-- +goose Down
DROP TABLE follows;
