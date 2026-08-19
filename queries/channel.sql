-- name: GetChannelDashboardByUserID :one
SELECT
    c.id,
    c.user_id,
    c.title,
    c.category,
    c.thumbnail_url,
    c.stream_key_hash,
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
RETURNING id, user_id, title, category, thumbnail_url, stream_key_hash, is_live, created_at;

-- name: UpdateChannelThumbnail :one
UPDATE channels
SET thumbnail_url = $2
WHERE user_id = $1
RETURNING id, user_id, title, category, thumbnail_url, stream_key_hash, is_live, created_at;
