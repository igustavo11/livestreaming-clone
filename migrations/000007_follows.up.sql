CREATE TABLE follows (
    follower_user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id uuid NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_user_id, channel_id)
);

CREATE INDEX idx_follows_channel_id ON follows(channel_id);
