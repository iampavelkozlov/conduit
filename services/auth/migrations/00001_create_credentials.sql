-- +goose Up
-- username is retained temporarily to make registration atomic while Profile
-- is split out. bio/image are compatibility columns required by the current
-- generated repository projection and are not owned or exposed by Auth.
CREATE TABLE users (
    id UUID PRIMARY KEY CONSTRAINT auth_users_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(id), 6) >> 4) = 8),
    email TEXT NOT NULL UNIQUE,
    username TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    bio TEXT,
    image TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY CONSTRAINT auth_sessions_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(id), 6) >> 4) = 8),
    refresh_token TEXT NOT NULL UNIQUE,
    jwt_id UUID NOT NULL CONSTRAINT auth_sessions_jwt_id_uuid_v8_check
        CHECK ((get_byte(uuid_send(jwt_id), 6) >> 4) = 8),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_jwt_id_idx ON sessions(jwt_id);
CREATE INDEX sessions_user_jwt_id_idx ON sessions(user_id, jwt_id);

-- +goose Down
DROP TABLE sessions;
DROP TABLE users;
