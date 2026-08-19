# HLS distribution via R2 + CDN from the MVP, not phase 3

The README roadmap originally placed object storage and CDN at phase 3, with the MVP serving HLS segments from the VPS via nginx. We pulled this forward: FFmpeg writes segments to a local staging volume, and a small Go uploader sidecar pushes each finished segment to Cloudflare R2 immediately; viewers get HLS through the Cloudflare edge.

The MVP bottleneck was never CPU or disk — it was the VPS uplink (~50 concurrent viewers at 4 Mbps on a 200 Mbps line). R2 has free egress and a generous free tier; live HLS segments are ephemeral, so storage cost is negligible. The price of pulling it forward is one extra sidecar and careful CDN cache headers (short TTL on playlists, immutable on segments), which is far cheaper than migrating delivery later.
