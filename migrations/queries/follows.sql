-- name: FollowUser :exec
INSERT INTO follows (follower_id, followee_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnfollowUser :exec
DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2;

-- name: IsFollowing :one
SELECT EXISTS (SELECT 1 FROM follows WHERE follower_id = $1 AND followee_id = $2);

-- name: ListFolloweeIDsByFollowerID :many
SELECT followee_id FROM follows WHERE follower_id = $1;

-- name: ListFollowingIDs :many
SELECT followee_id
FROM follows
WHERE follower_id = sqlc.arg(follower_id)
  AND followee_id = ANY(sqlc.arg(candidate_ids)::uuid[]);
