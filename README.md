# Live Streaming Platform — MVP

> A live streaming platform (Twitch/Kick-like reference) built in Go, with an architecture designed around three independent planes so the MVP can stay simple while still being able to scale without a rewrite.

## Table of contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Tech stack](#tech-stack)
- [MVP screens](#mvp-screens)
- [Functional requirements](#functional-requirements)
- [Non-functional requirements](#non-functional-requirements)
- [API surface](#api-surface)
- [MediaMTX contract](#mediamtx-contract)
- [Out of scope for the MVP](#out-of-scope-for-the-mvp)
- [Roadmap by phase](#roadmap-by-phase)
- [Provisioning — MVP → scale](#provisioning--mvp--scale)
- [MVP acceptance criteria](#mvp-acceptance-criteria)

See also: [CONTEXT.md](./CONTEXT.md) (ubiquitous language) and [docs/adr/](./docs/adr/) (architectural decisions).

---

## Overview

A streamer publishes video over RTMP (OBS), the platform transcodes and distributes it via HLS, and any visitor can watch in the browser with real-time chat. The MVP's goal is to validate the full pipeline — publish → transcode → watch → chat — with as few moving parts as possible, while still being built from day one on an architecture that can grow.

## Architecture

The system is split into three independent planes, each scaling at its own pace:

### Media plane
Handles video: RTMP ingest, transcoding, packaging, and distribution.

- **RTMP ingest** — MediaMTX, with publisher authentication delegated to the control plane via its blocking HTTP auth (`authMethod: http`).
- **Transcoding** — FFmpeg launched by MediaMTX's `runOnAvailable` hook (one process per stream; it receives SIGINT automatically when the stream ends). Job queue (`asynq` on Redis) arrives in phase 3 with multi-bitrate, when the queue earns its keep.
- **Object storage** — HLS segments are written to a local staging volume and pushed to Cloudflare R2 immediately by a small Go uploader sidecar.
- **CDN edge** — Cloudflare caches the `.m3u8`/`.ts` segments at the edge from day one; this is the cheapest layer to scale (static, cacheable files). Short TTL on playlists, immutable on segments.

### Control plane (Go)
A modular monolith serving the stateless REST API, the chat WebSocket, and background loops (live/offline reconciler). Package boundaries are the future seams for splitting into independent services (phase 6). Postgres is the source of truth.

- Auth, channels, stream keys, metadata.
- Stream key validation via MediaMTX's blocking auth callback (publish rejected on non-2xx).
- Live status reconciled every ~10s against MediaMTX's control API (`/v3/paths/list`), as a safety net for lost end-of-stream signals.
- Any API instance can answer any request — a prerequisite for horizontal scaling.

### Real-time plane
Chat and viewer count, over WebSocket (inside the monolith for now). Scales horizontally via Redis pub/sub, never relying on state held only in a single instance's memory. Anonymous viewers hold a read-only connection; the viewer count is the number of connected clients per channel, broadcast every ~15s.

### Entry point and load balancing
- HTTP/WS traffic (API + chat + frontend) goes through Caddy (automatic TLS on the VPS; a managed cloud LB later), routed by path.
- RTMP traffic doesn't go through a layer-7 LB — distribution is handled via DNS round-robin (MVP) or a layer-4 LB (at scale).

## Tech stack

| Layer | Technology |
|---|---|
| Ingest | MediaMTX (custom image with ffmpeg) |
| Transcode | FFmpeg (via MediaMTX hooks; asynq queue from phase 3) |
| API | Go (chi), modular monolith |
| Database | PostgreSQL (pgx + sqlc + golang-migrate) |
| Cache / pub-sub | Redis (chat pub/sub) |
| Auth | Session tokens (HttpOnly cookie, Postgres-backed), Google OAuth via `golang.org/x/oauth2`, argon2id |
| Email | Resend (password reset) |
| Object storage | Cloudflare R2 |
| CDN | Cloudflare |
| Edge/TLS | Caddy |
| Player | hls.js |
| Front | React (Vite, shadcn, TanStack Query) |
| Observability | `slog` structured JSON logs, Prometheus `/metrics` |
| Initial infra | Docker Compose on a single VPS |

## MVP screens

1. **Home / discovery** — grid of live channels, filterable by category.
2. **Channel page** — player + side chat + viewer count.
3. **Streamer dashboard** — stream key, metadata editing, live/offline status.
4. **Login / sign-up** — email/password + Google OAuth.

## Functional requirements

**Account and channel**
- Every registered user is a potential streamer: sign-up creates the User and their Channel together (1:1). There is no separate "streamer role".
- Sign-up collects email, username, and password. The username is unique, lowercase `[a-z0-9_]`, 3–25 chars, and becomes the channel's URL slug. Renaming is out of MVP scope.
- Login: email/password or Google. Google is a first-class identity: a Google login whose email matches an existing user links to that account instead of creating a duplicate; Google-first users pick their username in an onboarding step.
- Streamer generates and rotates their channel's stream key. Keys are stored only as hashes; a lost key cannot be recovered, only rotated (rotation invalidates the previous key immediately). The dashboard shows only a preview.
- Streamer edits channel metadata: title, category, thumbnail. Categories are a fixed enum in code (~10 options) for the MVP. Thumbnail is uploaded to R2.

**Broadcasting**
- The system validates the stream key at publish time via MediaMTX's blocking HTTP auth and rejects invalid keys or keys whose channel is already live.
- The system transcodes the incoming stream to a single HLS rendition: 720p30, x264 `veryfast`, AAC 128k.
- The system marks the channel as "live" when the publish is accepted, and "offline" when it ends or drops; because end-of-stream signals can be lost, status is periodically reconciled against the ingest server's active paths.

**Watching**
- Any visitor can watch a live channel via the channel's URL, without needing an account.
- The player plays back over HLS with basic play/pause/volume controls.
- The system lists the channels currently live (home/discovery), filterable by category.

**Chat**
- Logged-in users can send messages in a channel's chat; messages are ephemeral (pub/sub only, no history). Max 200 characters, rate-limited to 1 message/second per user.
- Everyone connected to the channel — including anonymous viewers in read-only mode — receives messages in real time.
- The viewer count is the number of real-time connections on the channel, broadcast to all connected clients every ~15s.

## Non-functional requirements

**Media plane**
- Acceptable latency for the MVP: standard HLS (10–30s of delay is fine; true low latency is a future phase via LL-HLS or WebRTC).
- Multiple simultaneous streams without one affecting another's transcoding (isolation per process/worker).

**Control plane**
- Stateless API; sessions live in Postgres so any instance can serve any request.
- Session tokens: opaque, in HttpOnly cookies, 30-day absolute expiry, revoked server-side on logout.
- Stream keys never appear in logs or in any API response outside the moment they're generated; a middleware redacts secrets from logs.
- Passwords always hashed with argon2id.

**Real-time plane**
- Automatic chat reconnection on the client if the WS connection drops.
- Chat broadcast via Redis pub/sub — never relying solely on a single instance's memory.

**General**
- Everything containerized (Docker) from the MVP onward.
- Structured logs (JSON, `slog`) across all services.
- Minimal observability: active streams and concurrent viewers visible via a Prometheus `/metrics` endpoint.
- Informal uptime for the MVP ("if it goes down, it comes back fast") — a formal SLA is a later-stage concern.

## API surface

```
Auth
  POST   /api/auth/signup              email + username + password
  POST   /api/auth/login
  POST   /api/auth/logout
  GET    /api/auth/me
  GET    /api/auth/google              redirect to Google
  GET    /api/auth/google/callback     links/creates; new users get a pending cookie
  POST   /api/auth/google/onboarding   new Google users pick their username
  POST   /api/auth/forgot-password     sends email via Resend
  POST   /api/auth/reset-password

Public (home/watch)
  GET    /api/channels?category=x      live channels (home)
  GET    /api/channels/{username}      metadata + live status + viewer count

Dashboard (authenticated)
  GET    /api/me/channel               key preview + metadata (title, category, thumbnail_url, is_live)
  PUT    /api/me/channel               title, category (fixed slugs: gaming, just_chatting, music, irl, esports, art, education, tech, cooking, other)
  POST   /api/me/channel/thumbnail     multipart field "thumbnail" (jpeg/png/webp, max 2MB → R2)
  POST   /api/me/channel/stream-key    rotation (invalidates previous)

Real-time
  WS     /ws/chat/{username}           send (auth) / receive (everyone)

Internal (shared-secret header, never logged)
  POST   /internal/mediamtx/auth       MediaMTX blocking auth (authMethod: http)
  POST   /internal/mediamtx/hook       stream-available / stream-unavailable hooks

Ops
  GET    /metrics                      Prometheus metrics
```

## MediaMTX contract

```
OBS → rtmp://host/live/{streamKey}

MediaMTX ──POST──▶ /internal/mediamtx/auth     (authMethod: http, blocking)
                   validates key hash + channel not already live → 2xx / 403

runOnAvailable    ─▶ POST /internal/mediamtx/hook {"event":"stream-available"}  → channel live
runOnUnavailable  ─▶ POST /internal/mediamtx/hook {"event":"stream-unavailable"} → channel offline

Reconciler: polls MediaMTX /v3/paths/list every 30s as a safety net
All /internal/* endpoints protected by INTERNAL_SECRET header
```

## Out of scope for the MVP

VOD, clips, following channels, "went live" notifications, chat moderation (mute/ban), email verification at sign-up, username renaming. Named here so they don't fall off the radar, but they're not part of the first delivery.

## Roadmap by phase

0. **MVP** — end-to-end pipeline working (publish → transcode → watch), no chat yet.
1. **Control plane** — sign-up, channels, stream keys, metadata.
2. **Real-time chat** — WebSocket + Redis pub/sub, viewer count.
3. **Media plane scaling** — multi-bitrate (ABR) via an asynq job queue, origin/transcode hardening.
4. **VOD** — automatic recording, post-processing, long-term storage.
5. **Discovery and social** — follow channels, notifications, clips.
6. **Observability and horizontal scaling** — metrics, worker autoscaling, multiple ingest nodes, API/WS split into independent services along the package seams.

## Provisioning — MVP → scale

- **MVP (phases 0–2)**: a single VPS, everything in Docker Compose:

```yaml
services:
  mediamtx     # RTMP ingest :1935; custom image: mediamtx + ffmpeg + curl
  server       # Go monolith: API + WS + reconciler
  uploader     # watches the HLS staging volume → R2
  postgres
  redis        # chat pub/sub
  caddy        # automatic TLS; /api and /ws → server; / → web
volumes:
  hls          # shared: mediamtx (FFmpeg writes) ↔ uploader (reads/uploads)
```

  FFmpeg runs inside the `mediamtx` container (it is launched by its hooks); MediaMTX itself never exposes HLS publicly — only RTMP. The Vite frontend joins the compose as `web`.
- **Transition (phase 3)**: Postgres moves to a managed service (Cloud SQL/RDS) with a read replica.
- **Scale (phase 4+)**: transcode workers scale via queue-based autoscaling, API/WS become independent services behind the LB (Cloud Run/ECS/k8s), R2 + Cloudflare already carry all viewer traffic from day one.

Postgres scaling, in this order: read replica → connection pooling (PgBouncer) → partitioning on high-volume tables. Sharding would only come in at a scale far beyond the MVP.

## MVP acceptance criteria

- [ ] A streamer can publish via OBS with a validated stream key.
- [ ] The stream shows up as "live" on the home page within a few seconds of publishing.
- [ ] A visitor without an account can watch the live stream via the web player.
- [ ] Chat works in real time with automatic reconnection.
- [ ] The whole stack comes up with `docker compose up`.
- [ ] No secret (stream key, password, shared secret) ever appears in logs.
