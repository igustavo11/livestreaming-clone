# Auth lives in the Go control plane, not in the frontend

The frontend is Next.js, where libraries like better-auth would make email/password + Google OAuth nearly free. We still chose to implement auth in the Go API (control plane): Google OAuth via `golang.org/x/oauth2`, argon2id password hashing, opaque session tokens in HttpOnly cookies backed by a Postgres `sessions` table (30-day absolute expiry).

The README makes the control plane the single authority for auth, channels, and stream keys, and the Go chat server must authenticate WebSocket connections anyway. Putting the auth authority in Next.js would split identity ownership across two runtimes and force the Go services to verify a foreign session scheme. The cost of hand-rolling was small compared to that coupling.
