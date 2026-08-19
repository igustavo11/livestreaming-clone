# Modular monolith first; services split later along package seams

The control plane (REST API), real-time plane (chat WebSocket), and background loops (live/offline reconciler) ship as a single Go binary with strict package boundaries (`internal/auth`, `channel`, `stream`, `chat`, ...). Only the HLS uploader runs as a separate binary, because it lives next to the media volume.

Splitting API and WS into separate services at day one would duplicate auth/session middleware and add deployment surface with no MVP traffic to justify it. The package boundaries are the future seams: when horizontal scaling demands it (roadmap phase 6), a package can be extracted into its own binary without rewriting. The README's "API/WS become independent services" remains the destination — this only defers the split.
