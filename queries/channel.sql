-- name: GetChannelDashboardByUserID :one
SELECT
    c.id,
    c.user_id,
    c.title,
    c.category,
    c.thumbnail_url,
    c.avatar_url,
    c.stream_key_hash,
    c.stream_key_preview,
    c.is_live,
    c.created_at,
    u.username
FROM channels c
JOIN users u ON u.id = c.user_id
WHERE c.user_id = $1;

-- name: UpdateChannelMetadata :one
UPDATE channels
SET title = $2, category = $3
WHERE user_id = $1
RETURNING id, user_id, title, category, thumbnail_url, avatar_url, stream_key_hash, stream_key_preview, is_live, created_at;

-- name: UpdateChannelThumbnail :one
UPDATE channels
SET thumbnail_url = $2
WHERE user_id = $1
RETURNING id, user_id, title, category, thumbnail_url, avatar_url, stream_key_hash, stream_key_preview, is_live, created_at;

-- name: UpdateChannelAvatar :one
UPDATE channels
SET avatar_url = $2
WHERE user_id = $1
RETURNING id, user_id, title, category, thumbnail_url, avatar_url, stream_key_hash, stream_key_preview, is_live, created_at;

-- name: UpdateChannelStreamKey :one
UPDATE channels
SET stream_key_hash = $2, stream_key_preview = $3
WHERE user_id = $1
RETURNING id, user_id, title, category, thumbnail_url, avatar_url, stream_key_hash, stream_key_preview, is_live, created_at;

-- name: GetChannelByStreamKeyHash :one
SELECT id, user_id, title, category, thumbnail_url, stream_key_hash, stream_key_preview, is_live, created_at
FROM channels
WHERE stream_key_hash = $1;

-- name: SetChannelLive :exec
UPDATE channels
SET is_live = $2
WHERE id = $1;

-- name: GetChannelUsernameByStreamKeyHash :one
SELECT u.username
FROM channels c
JOIN users u ON u.id = c.user_id
WHERE c.stream_key_hash = $1;

-- name: GetAllLiveChannels :many
SELECT id, user_id, title, category, thumbnail_url, stream_key_hash, stream_key_preview, is_live, created_at
FROM channels
WHERE is_live = true;

-- name: ListLiveChannelsFiltered :many
-- category and search are optional (pass NULL to skip). search matches
-- username or title, case-insensitively. Callers fetch limit+1 rows to
-- cheaply derive "has more" without a separate COUNT query.
SELECT c.id, c.user_id, u.username, c.title, c.category, c.thumbnail_url, c.avatar_url, c.is_live, c.created_at,
    (SELECT COUNT(*) FROM follows f WHERE f.channel_id = c.id) AS follower_count
FROM channels c
JOIN users u ON u.id = c.user_id
WHERE c.is_live = true
  AND (sqlc.narg('category')::text IS NULL OR c.category = sqlc.narg('category'))
  AND (
    sqlc.narg('search')::text IS NULL
    OR u.username ILIKE '%' || sqlc.narg('search') || '%'
    OR c.title ILIKE '%' || sqlc.narg('search') || '%'
  )
ORDER BY c.created_at DESC
LIMIT $1 OFFSET $2;

-- name: GetPublicChannelByUsername :one
SELECT c.id, c.user_id, u.username, c.title, c.category, c.thumbnail_url, c.avatar_url, c.is_live, c.created_at,
    (SELECT COUNT(*) FROM follows f WHERE f.channel_id = c.id) AS follower_count
FROM channels c
JOIN users u ON u.id = c.user_id
WHERE u.username = $1;

-- name: GetActiveStreamsCount :one
SELECT COUNT(*) FROM channels WHERE is_live = true;
