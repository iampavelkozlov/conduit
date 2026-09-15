-- name: CreateSession :exec
INSERT INTO sessions (id, refresh_token, jwt_id, user_id, created_at)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSessionByUserIDAndJWTIDAndRefreshToken :one
SELECT id, refresh_token, jwt_id, user_id, created_at
FROM sessions
WHERE user_id = $1 AND jwt_id = $2 AND refresh_token = $3;

-- name: RotateSession :execrows
WITH deleted AS (
    DELETE FROM sessions
    WHERE sessions.id = sqlc.arg(old_session_id)
    RETURNING 1
)
INSERT INTO sessions (id, refresh_token, jwt_id, user_id, created_at)
SELECT
    sqlc.arg(new_session_id),
    sqlc.arg(refresh_token),
    sqlc.arg(jwt_id),
    sqlc.arg(user_id),
    sqlc.arg(created_at)
FROM deleted;
