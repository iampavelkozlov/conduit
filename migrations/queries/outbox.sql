-- name: ClaimOutboxEvents :many
WITH selected AS (
    SELECT id
    FROM outbox_events
    WHERE published_at IS NULL
      AND available_at <= NOW()
      AND (locked_at IS NULL OR locked_at < NOW() - sqlc.arg(lock_timeout)::INTERVAL)
    ORDER BY occurred_at, id
    FOR UPDATE SKIP LOCKED
    LIMIT sqlc.arg(batch_size)
)
UPDATE outbox_events AS event
SET locked_at = NOW(), attempts = attempts + 1, last_error = NULL
FROM selected
WHERE event.id = selected.id
RETURNING event.id, event.topic, event.event_key, event.event_type,
          event.event_version, event.source, event.aggregate_id, event.payload,
          event.occurred_at, event.attempts;

-- name: MarkOutboxPublished :execrows
UPDATE outbox_events
SET published_at = NOW(), locked_at = NULL, last_error = NULL
WHERE id = $1 AND published_at IS NULL;

-- name: ReleaseOutboxEvent :execrows
UPDATE outbox_events
SET locked_at = NULL,
    available_at = NOW() + sqlc.arg(retry_delay)::INTERVAL,
    last_error = LEFT(sqlc.arg(last_error), 1024)
WHERE id = sqlc.arg(id) AND published_at IS NULL;

-- name: ClaimInboxEvent :one
INSERT INTO inbox_events (event_id, consumer)
VALUES (sqlc.arg(event_id), sqlc.arg(consumer))
ON CONFLICT (event_id, consumer) DO UPDATE
SET locked_at = NOW(), last_error = NULL
WHERE inbox_events.processed_at IS NULL
  AND inbox_events.locked_at < NOW() - sqlc.arg(lock_timeout)::INTERVAL
RETURNING TRUE::BOOLEAN AS claimed;

-- name: CompleteInboxEvent :execrows
UPDATE inbox_events
SET processed_at = NOW(), last_error = NULL
WHERE event_id = sqlc.arg(event_id)
  AND consumer = sqlc.arg(consumer)
  AND processed_at IS NULL;

-- name: ReleaseInboxEvent :execrows
UPDATE inbox_events
SET locked_at = TO_TIMESTAMP(0), last_error = LEFT(sqlc.arg(last_error), 1024)
WHERE event_id = sqlc.arg(event_id)
  AND consumer = sqlc.arg(consumer)
  AND processed_at IS NULL;
