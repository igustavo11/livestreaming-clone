-- name: CreateIdentity :exec
INSERT INTO identities (provider, subject, user_id)
VALUES ($1, $2, $3);

-- name: GetIdentity :one
SELECT * FROM identities WHERE provider = $1 AND subject = $2;
