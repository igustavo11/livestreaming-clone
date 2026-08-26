// The Go server already renders a correct, live-status-aware player at
// GET /player/{username} (offline fallback, signed CDN URL from a
// backend-only env var). Embedding it avoids re-deriving the HLS URL and
// re-implementing hls.js client-side for logic the backend already has.
export function PlayerEmbed({ username }: { username: string }) {
	return (
		<div className="relative aspect-video w-full bg-black">
			<iframe
				src={`/player/${encodeURIComponent(username)}`}
				title={`Player de ${username}`}
				className="size-full border-0"
				allow="autoplay"
				allowFullScreen
			/>
		</div>
	);
}
