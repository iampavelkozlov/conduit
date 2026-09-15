-- name: FavoriteArticle :exec
INSERT INTO article_favorites (article_id, user_id) VALUES ($1, $2) ON CONFLICT DO NOTHING;

-- name: UnfavoriteArticle :exec
DELETE FROM article_favorites WHERE article_id = $1 AND user_id = $2;

-- name: ListFavoriteArticleIDsByUserID :many
SELECT article_id FROM article_favorites WHERE user_id = $1;

-- name: CountFavoritesByArticleIDs :many
SELECT article_id, COUNT(*) AS favorites_count
FROM article_favorites
WHERE article_id = ANY(sqlc.arg(article_ids)::uuid[])
GROUP BY article_id;

-- name: ListFavoriteArticleIDsByUserIDAndArticleIDs :many
SELECT article_id
FROM article_favorites
WHERE user_id = sqlc.arg(user_id)
  AND article_id = ANY(sqlc.arg(article_ids)::uuid[]);
