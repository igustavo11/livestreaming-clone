# web

Frontend for guguinha: React + TypeScript on Vite, Tailwind, shadcn/ui.

## Scripts

- `npm run dev` — dev server with HMR
- `npm run build` — type-check + production build
- `npm run test` — Vitest
- `npm run check` — Biome format + lint (writes fixes)
- `npm run lint` — Biome lint only

## Docker

Runs as the `web` service in the repo's root `docker-compose.yml`, proxied by Caddy at `/`. `/api` and `/ws` are proxied straight to the Go `server` container both by Caddy and by Vite's own dev-server proxy (`vite.config.ts`), so the session cookie is always same-origin — no CORS setup needed.
