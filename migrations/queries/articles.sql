-- name: CreateArticle :one
INSERT INTO articles (id, author_id, slug, title, description, body)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id;

-- name: GetArticleBySlug :one
SELECT id, author_id, slug, title, description, body, created_at, updated_at FROM articles WHERE slug = $1;

-- name: GetArticleIDBySlug :one
SELECT id FROM articles WHERE slug = $1;

-- name: ListArticles :many
SELECT id, author_id, slug, title, description, body, created_at, updated_at
FROM articles
WHERE (NOT sqlc.arg(filter_author_ids)::boolean OR author_id = ANY(sqlc.arg(author_ids)::uuid[]))
  AND (NOT sqlc.arg(filter_article_ids)::boolean OR id = ANY(sqlc.arg(article_ids)::uuid[]))
ORDER BY created_at DESC, id DESC
LIMIT sqlc.arg(page_limit) OFFSET sqlc.arg(page_offset);

-- name: CountArticles :one
SELECT COUNT(*)
FROM articles
WHERE (NOT sqlc.arg(filter_author_ids)::boolean OR author_id = ANY(sqlc.arg(author_ids)::uuid[]))
  AND (NOT sqlc.arg(filter_article_ids)::boolean OR id = ANY(sqlc.arg(article_ids)::uuid[]));

-- name: UpdateArticle :one
UPDATE articles
SET title = $2,
    description = $3,
    body = $4,
    updated_at = NOW()
WHERE slug = $1
RETURNING id;

-- name: DeleteArticleBySlugAndAuthorID :execrows
DELETE FROM articles WHERE slug = $1 AND author_id = $2;
