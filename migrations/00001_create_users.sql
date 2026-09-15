-- +goose Up
CREATE TABLE users (
    id UUID PRIMARY KEY CONSTRAINT users_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(id), 6) >> 4) = 8),
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    bio TEXT,
    image TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE users;
