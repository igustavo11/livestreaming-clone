-- name: CreateUser :one
INSERT INTO users (email, username, password_hash)
VALUES ($1, $2, $3)
RETURNING id, email, username, created_at;

-- name: CreateChannelForUser :one
INSERT INTO channels (user_id)
VALUES ($1)
RETURNING id, user_id, title, category, thumbnail_url, is_live, created_at;

-- name: GetUserByEmail :one
SELECT * FROM users WHERE email = $1;

-- name: GetUserByUsername :one
SELECT * FROM users WHERE username = $1;

-- name: GetChannelByUserID :one
SELECT * FROM channels WHERE user_id = $1;

-- name: CreateSession :exec
INSERT INTO sessions (token_hash, user_id, expires_at)
VALUES ($1, $2, $3);

-- name: GetSessionUser :one
SELECT u.id, u.email, u.username, s.expires_at
FROM sessions s
JOIN users u ON u.id = s.user_id
WHERE s.token_hash = $1;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE token_hash = $1;
