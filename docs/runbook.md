# Runbook — Live Streaming Platform MVP

This runbook documents how to bring the full stack up, publish a stream, watch it, and operate the system. It is the acceptance document for issue #13.

## Prerequisites

- Docker and Docker Compose v2
- `cp .env.example .env` and fill the required vars (see below)
- Ports free: `80`, `443`, `1935` (RTMP), `8080` (server, via Caddy)

## Environment

```bash
cp .env.example .env
# Required
DATABASE_URL=postgres://guguinha:guguinha@postgres:5432/guguinha?sslmode=disable
AUTH_SECRET=change-me-32chars
INTERNAL_SECRET=change-me-internal
# Optional (R2, Resend, Google OAuth)
R2_ACCOUNT_ID=
R2_ACCESS_KEY_ID=
R2_SECRET_ACCESS_KEY=
R2_BUCKET=
R2_PUBLIC_BASE_URL=https://<bucket>.r2.cloudflarestorage.com
RESEND_API_KEY=
RESEND_FROM=noreply@localhost
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
PUBLIC_BASE_URL=http://localhost
MEDIAMTX_URL=http://mediamtx:9997
REDIS_ADDR=redis:6379
```

`AUTH_SECRET` and `INTERNAL_SECRET` must be long, random strings. `INTERNAL_SECRET` is the shared secret between `server` and `mediamtx` for `/internal/mediamtx/*`.

## Bring the stack up

```bash
docker compose up --build -d
docker compose ps
docker compose logs -f server   # JSON logs, secrets redacted
docker compose logs -f mediamtx
```

Health:

```bash
curl http://localhost/healthz
# {"status":"ok"}
curl http://localhost/metrics | head -20
# livestreaming_active_streams, livestreaming_viewers, http_requests_total
```

All services have healthchecks (`postgres`, `redis`, `mediamtx` via `curl`).

## Streamer flow

1. **Sign up / login**

```bash
curl -c cookies.txt -H "Content-Type: application/json" \
  -d '{"email":"streamer@example.com","username":"streamer","password":"password123"}' \
  http://localhost/api/auth/signup
```

2. **Get stream key (rotation, shown once)**

```bash
curl -b cookies.txt -X POST http://localhost/api/me/channel/stream-key
# {"stream_key":"live_...","channel":{...,"stream_key_preview":"live_****abcd",...}}
```

The dashboard (`GET /api/me/channel`) shows only `stream_key_preview`.

3. **Publish via OBS**

- Service: Custom
- Server: `rtmp://localhost/live`
- Stream Key: the `live_...` value from the previous step (the full key, not the preview)

MediaMTX will call `POST /internal/mediamtx/auth` with the key in the path (`/live/<key>`). The server validates the hash and that the channel is not already live, and returns `2xx` to allow, or `403` to reject.

On `runOnAvailable`, MediaMTX calls `POST /internal/mediamtx/hook` with `{"event":"stream-available","path":"live/<key>"}` and the server marks the channel `is_live=true`. On `runOnUnavailable`, it marks `offline`. A reconciler polls `MEDIAMTX_URL/v3/paths/list` every 30s as a safety net.

4. **Verify live on home**

```bash
curl http://localhost/api/channels | jq
# [{"username":"streamer","title":"","is_live":true,"viewer_count":0,...}]
curl "http://localhost/api/channels?category=gaming"
```

The channel appears as live within seconds of publishing.

## Watch as visitor (no account)

```bash
curl http://localhost/api/channels/streamer | jq
# {"channel":{"username":"streamer","is_live":true,"viewer_count":1,...}}

# Player page (hls.js)
open http://localhost/player/streamer
# The HTML is rendered by the server at GET /player/{username} and contains
# <video> with hls.js loading https://<R2_PUBLIC_BASE_URL>/hls/<username>/index.m3u8
```

The HLS path is `hls/<username>/index.m3u8` on R2, cached by Cloudflare. Segments are uploaded by the `hlsupload` sidecar from the shared `hls_staging` volume.

## Chat

- **WS endpoint:** `ws://localhost/ws/chat/{username}`
- Anonymous can connect read-only; only authenticated users (cookie `session`) can send.
- Messages are ephemeral (Redis pub/sub, no history), max 200 chars, 1 msg/sec per user.
- Viewer count is broadcast every ~15s as `{"type":"viewer_count","viewer_count":N}` and is also exposed on `GET /api/channels/{username}`.

Test with `wscat`:

```bash
# anonymous viewer
wscat -c ws://localhost/ws/chat/streamer
# authenticated sender (use cookies.txt from login)
wscat -c ws://localhost/ws/chat/streamer --header "Cookie: $(grep session cookies.txt | awk '{print $7"="$8}')"
> {"message":"hello"}
< {"type":"chat","username":"streamer","message":"hello"}
```

Reconnection is automatic on the client (hls.js and chat both handle drops).

## Thumbnail and metadata

```bash
curl -b cookies.txt -X PUT -H "Content-Type: application/json" \
  -d '{"title":"My Stream","category":"gaming"}' http://localhost/api/me/channel

curl -b cookies.txt -X POST -F "thumbnail=@thumb.jpg" http://localhost/api/me/channel/thumbnail
```

Categories: `gaming, just_chatting, music, irl, esports, art, education, tech, cooking, other`.

## Operations

**Logs (JSON, redacted):**

```bash
docker compose logs -f --tail=100 server | jq
# No stream_key, password, or INTERNAL_SECRET ever appears;
# the redact handler replaces them with [REDACTED]
```

**Metrics:**

```bash
curl http://localhost/metrics
```

**Restart / update:**

```bash
docker compose down
git pull
docker compose up --build -d
```

**Rotate secrets:**

- `AUTH_SECRET` change invalidates all sessions (users must re-login).
- `INTERNAL_SECRET` must be changed in both `server` and `mediamtx` env.
- Stream key rotation is per-channel via `POST /api/me/channel/stream-key`.

**Troubleshooting:**

- `docker compose ps` shows health.
- `curl http://localhost/healthz` should be `ok`; if `degraded`, check `postgres`.
- MediaMTX logs: `docker compose logs mediamtx` — look for `auth` and `hook` calls.
- If a channel stays `is_live=true` after OBS stops, the reconciler will correct it within 30s (it polls `/v3/paths/list`).
- R2 upload failures appear in `hlsupload` logs; HLS will still be available locally via staging volume for local dev.

## Acceptance verification

Run the E2E suite (testcontainers, no external deps):

```bash
go test ./test/e2e -run TestMVP -count=1 -v
```

It verifies: signup → stream-key rotation → ingest auth (valid/invalid) → hook live → home lists live → player contains HLS URL → chat send/receive + reconnect → no secret in logs → compose health.

CI runs `go vet + go build + go test ./...` on every push.
