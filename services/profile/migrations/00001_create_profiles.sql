-- +goose Up
CREATE TABLE profiles (
    user_id UUID PRIMARY KEY CONSTRAINT profiles_user_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(user_id), 6) >> 4) = 8),
    username TEXT NOT NULL UNIQUE,
    bio TEXT,
    image TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- +goose Down
DROP TABLE profiles;
