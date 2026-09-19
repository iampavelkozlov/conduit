-- name: CreateProfile :one
INSERT INTO profiles (user_id, username)
VALUES ($1, $2)
ON CONFLICT (user_id) DO NOTHING
RETURNING user_id, username, bio, image, created_at, updated_at;

-- name: GetProfileByID :one
SELECT user_id, username, bio, image, created_at, updated_at
FROM profiles
WHERE user_id = $1;

-- name: GetProfileByUsername :one
SELECT user_id, username, bio, image, created_at, updated_at
FROM profiles
WHERE username = $1;

-- name: ListProfilesByIDs :many
SELECT user_id, username, bio, image, created_at, updated_at
FROM profiles
WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[]);

-- name: UpdateProfile :one
UPDATE profiles
SET username = CASE WHEN sqlc.arg(set_username)::boolean THEN sqlc.arg(username)::text ELSE username END,
    bio = CASE WHEN sqlc.arg(set_bio)::boolean THEN sqlc.narg(bio)::text ELSE bio END,
    image = CASE WHEN sqlc.arg(set_image)::boolean THEN sqlc.narg(image)::text ELSE image END,
    updated_at = NOW()
WHERE user_id = sqlc.arg(user_id)
RETURNING user_id, username, bio, image, created_at, updated_at;
