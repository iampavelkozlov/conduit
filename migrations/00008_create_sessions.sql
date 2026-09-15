-- +goose Up
CREATE TABLE sessions (
    id UUID PRIMARY KEY CONSTRAINT sessions_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(id), 6) >> 4) = 8),
    refresh_token TEXT NOT NULL UNIQUE,
    jwt_id UUID NOT NULL CONSTRAINT sessions_jwt_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(jwt_id), 6) >> 4) = 8),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_jwt_id_idx ON sessions(jwt_id);
CREATE INDEX sessions_user_jwt_id_idx ON sessions(user_id, jwt_id);

-- +goose Down
DROP TABLE sessions;
