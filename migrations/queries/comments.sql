-- name: CreateComment :one
INSERT INTO comments (id, article_id, author_id, body)
VALUES ($1, $2, $3, $4)
RETURNING id, author_id, body, created_at, updated_at;

-- name: ListCommentsByArticleID :many
SELECT id, author_id, body, created_at, updated_at
FROM comments
WHERE article_id = $1
ORDER BY created_at DESC, id DESC;

-- name: GetCommentAuthorIDByIDAndArticleID :one
SELECT author_id FROM comments
WHERE id = $1 AND article_id = $2;

-- name: DeleteCommentByIDAndArticleIDAndAuthorID :execrows
DELETE FROM comments
WHERE id = $1 AND article_id = $2 AND author_id = $3;
