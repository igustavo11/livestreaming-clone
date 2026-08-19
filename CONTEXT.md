# Live Streaming Platform

Platform where streamers broadcast live video (RTMP → HLS) and viewers watch with real-time chat. Twitch/Kick reference.

## Language

### Accounts

**User**:
A registered person. Can watch, chat, and broadcast. Every User is a potential streamer — there is no separate "streamer role".
_Avoid_: account, viewer account, streamer account

**Channel**:
A User's broadcast identity, created automatically at sign-up (1:1 with User). Holds stream key, metadata, and live status.
_Avoid_: profile, stream

**Username**:
A User's unique public handle, chosen at sign-up. It is the Channel's URL slug. Renaming is out of MVP scope.
_Avoid_: handle, display name, nickname

**Identity**:
A credential linking an external provider (e.g. Google) or the local password to a User. Email is the join key: a Google login whose email matches an existing User links to that User instead of creating a new one.
_Avoid_: provider account, social login, OAuth account

### Media

**Stream**:
An active publishing session — from the moment an RTMP publish is accepted until it ends or drops. A Channel is "live" while it has a Stream.
_Avoid_: broadcast (for the session), transmission

**Stream Key**:
A secret (`live_` + random base32) that authorizes publishing to a Channel. Stored only as a hash; if lost it cannot be recovered, only rotated — rotation invalidates the previous key immediately.
_Avoid_: token, publish key

**Live Status**:
A Channel's public state, `live` or `offline`. Set `live` when a publish is accepted, `offline` when the stream ends. Because end-of-stream signals can be lost, status is periodically reconciled against the ingest server's actual active paths.
_Avoid_: online/offline, streaming state

### Real-time

**Chat Message**:
An ephemeral message sent by a logged-in User to a Channel's chat. Exists only in transit (pub/sub); there is no history to fetch or recover.
_Avoid_: message log, chat history

**Viewer Count**:
The number of clients connected to a Channel's real-time connection, broadcast to all connected clients periodically (~15s). Anonymous viewers are included because they hold a read-only connection.
_Avoid_: concurrent users, audience size
