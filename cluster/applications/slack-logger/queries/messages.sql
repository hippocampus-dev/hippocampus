-- name: InsertMessage :exec
INSERT INTO encrypted_messages (
    channel_name,
    message_ts,
    thread_ts,
    salt,
    encrypted_data,
    timestamp
) VALUES (?, ?, ?, ?, ?, ?);

-- name: GetMessagesByChannelName :many
SELECT
    id,
    channel_name,
    message_ts,
    thread_ts,
    salt,
    encrypted_data,
    timestamp,
    created_at
FROM encrypted_messages
WHERE channel_name = ?
  AND (thread_ts IS NULL OR thread_ts = message_ts)
  AND (
    CAST(sqlc.narg('since_time') AS DATETIME) IS NULL
    OR timestamp >= CASE
      WHEN CAST(sqlc.narg('inclusive') AS SIGNED) = 1
      THEN CAST(sqlc.narg('since_time') AS DATETIME)
      ELSE DATE_ADD(CAST(sqlc.narg('since_time') AS DATETIME), INTERVAL 1 MICROSECOND)
    END
  )
  AND (
    CAST(sqlc.narg('until_time') AS DATETIME) IS NULL
    OR timestamp <= CASE
      WHEN CAST(sqlc.narg('inclusive') AS SIGNED) = 1
      THEN CAST(sqlc.narg('until_time') AS DATETIME)
      ELSE DATE_SUB(CAST(sqlc.narg('until_time') AS DATETIME), INTERVAL 1 MICROSECOND)
    END
  )
ORDER BY timestamp DESC
LIMIT ? OFFSET ?;

-- name: UpdateMessage :exec
UPDATE encrypted_messages
SET thread_ts = ?,
    salt = ?,
    encrypted_data = ?,
    timestamp = ?
WHERE channel_name = ? AND message_ts = ?;

-- name: DeleteMessage :exec
DELETE FROM encrypted_messages
WHERE channel_name = ? AND message_ts = ?;

-- name: GetMessagesByThreadTs :many
SELECT
    id,
    channel_name,
    message_ts,
    thread_ts,
    salt,
    encrypted_data,
    timestamp,
    created_at
FROM encrypted_messages
WHERE channel_name = ?
  AND (thread_ts = ? OR (message_ts = ? AND (thread_ts IS NULL OR thread_ts = message_ts)))
  AND (
    CAST(sqlc.narg('since_time') AS DATETIME) IS NULL
    OR timestamp >= CASE
      WHEN CAST(sqlc.narg('inclusive') AS SIGNED) = 1
      THEN CAST(sqlc.narg('since_time') AS DATETIME)
      ELSE DATE_ADD(CAST(sqlc.narg('since_time') AS DATETIME), INTERVAL 1 MICROSECOND)
    END
  )
  AND (
    CAST(sqlc.narg('until_time') AS DATETIME) IS NULL
    OR timestamp <= CASE
      WHEN CAST(sqlc.narg('inclusive') AS SIGNED) = 1
      THEN CAST(sqlc.narg('until_time') AS DATETIME)
      ELSE DATE_SUB(CAST(sqlc.narg('until_time') AS DATETIME), INTERVAL 1 MICROSECOND)
    END
  )
ORDER BY timestamp ASC
LIMIT ? OFFSET ?;
