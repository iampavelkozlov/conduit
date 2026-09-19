-- name: CreateUser :one
INSERT INTO users (id, email, username, password_hash, bio, image)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, username, password_hash, bio, image, created_at, updated_at;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- name: GetUserByID :one
SELECT id, email, username, password_hash, bio, image, created_at, updated_at
FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, username, password_hash, bio, image, created_at, updated_at
FROM users WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET email = CASE WHEN sqlc.arg(set_email)::boolean THEN sqlc.arg(email)::text ELSE email END,
    username = CASE WHEN sqlc.arg(set_username)::boolean THEN sqlc.arg(username)::text ELSE username END,
    password_hash = CASE WHEN sqlc.arg(set_password)::boolean THEN sqlc.arg(password_hash)::text ELSE password_hash END,
    bio = CASE WHEN sqlc.arg(set_bio)::boolean THEN sqlc.narg(bio)::text ELSE bio END,
    image = CASE WHEN sqlc.arg(set_image)::boolean THEN sqlc.narg(image)::text ELSE image END,
    updated_at = NOW()
WHERE id = sqlc.arg(id)
RETURNING id, email, username, password_hash, bio, image, created_at, updated_at;

-- name: CreateSession :exec
INSERT INTO sessions (id, refresh_token, jwt_id, user_id, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSessionByUserIDAndJWTIDAndRefreshToken :one
SELECT id, refresh_token, jwt_id, user_id, created_at
FROM sessions
WHERE user_id = $1 AND jwt_id = $2 AND refresh_token = $3;

-- name: RotateSession :execrows
WITH deleted AS (
    DELETE FROM sessions WHERE sessions.id = sqlc.arg(old_session_id) RETURNING 1
)
INSERT INTO sessions (id, refresh_token, jwt_id, user_id, created_at)
SELECT sqlc.arg(new_session_id), sqlc.arg(refresh_token), sqlc.arg(jwt_id),
       sqlc.arg(user_id), sqlc.arg(created_at)
FROM deleted;
