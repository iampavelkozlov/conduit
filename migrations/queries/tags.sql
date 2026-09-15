-- name: UpsertTags :many
INSERT INTO tags (id, name)
SELECT ids.id, names.name
FROM unnest(sqlc.arg(ids)::uuid[]) WITH ORDINALITY AS ids(id, position)
JOIN unnest(sqlc.arg(names)::text[]) WITH ORDINALITY AS names(name, position) USING (position)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id, name, created_at, updated_at;

-- name: ListTags :many
SELECT name FROM tags ORDER BY name ASC;

-- name: AttachTagsToArticle :exec
INSERT INTO article_tags (article_id, tag_id)
SELECT sqlc.arg(article_id), unnest(sqlc.arg(tag_ids)::uuid[])
ON CONFLICT DO NOTHING;

-- name: DeleteArticleTags :exec
DELETE FROM article_tags WHERE article_id = $1;

-- name: ListArticleTagRelationsByArticleIDs :many
SELECT article_id, tag_id
FROM article_tags
WHERE article_id = ANY(sqlc.arg(article_ids)::uuid[]);

-- name: GetTagIDByName :one
SELECT id FROM tags WHERE name = $1;

-- name: ListTagsByIDs :many
SELECT id, name FROM tags WHERE id = ANY(sqlc.arg(ids)::uuid[]) ORDER BY name ASC;

-- name: ListArticleIDsByTagID :many
SELECT article_id FROM article_tags WHERE tag_id = $1;
