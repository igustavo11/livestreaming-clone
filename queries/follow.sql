-- name: FollowChannel :exec
INSERT INTO follows (follower_user_id, channel_id)
VALUES ($1, $2)
ON CONFLICT DO NOTHING;

-- name: UnfollowChannel :exec
DELETE FROM follows
WHERE follower_user_id = $1 AND channel_id = $2;

-- name: GetFollowerCount :one
SELECT COUNT(*) FROM follows WHERE channel_id = $1;

-- name: IsFollowing :one
SELECT EXISTS(
    SELECT 1 FROM follows WHERE follower_user_id = $1 AND channel_id = $2
);
