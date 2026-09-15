-- name: CreateUser :one
INSERT INTO users (id, email, username, password_hash, bio, image)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, email, username, password_hash, bio, image, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, username, password_hash, bio, image, created_at, updated_at FROM users WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, username, password_hash, bio, image, created_at, updated_at FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT id, email, username, password_hash, bio, image, created_at, updated_at FROM users WHERE username = $1;

-- name: GetUserIDByUsername :one
SELECT id FROM users WHERE username = $1;

-- name: ListProfilesByIDs :many
SELECT id, username, bio, image
FROM users
WHERE id = ANY(sqlc.arg(ids)::uuid[]);

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
