import { useCallback, useEffect, useRef, useState } from "react";

export type ChatMessage = {
	id: string;
	username: string;
	text: string;
};

type IncomingMessage =
	| { type: "chat"; username: string; message: string }
	| { type: "error"; error: string }
	| { type: "viewer_count"; viewer_count: number };

const MAX_MESSAGES = 200;
const RECONNECT_DELAY_MS = 2000;

// One WebSocket per mount, with a fixed-delay reconnect on drop — no
// exponential backoff, this is a chat sidebar, not a critical channel.
export function useChatSocket(channelUsername: string) {
	const [messages, setMessages] = useState<ChatMessage[]>([]);
	const [connected, setConnected] = useState(false);
	const [sendError, setSendError] = useState<string | null>(null);
	const [liveViewerCount, setLiveViewerCount] = useState<number | null>(null);
	const socketRef = useRef<WebSocket | null>(null);

	useEffect(() => {
		let cancelled = false;
		let retryTimeout: ReturnType<typeof setTimeout> | undefined;
		setMessages([]);

		function connect() {
			const protocol = window.location.protocol === "https:" ? "wss" : "ws";
			const socket = new WebSocket(
				`${protocol}://${window.location.host}/ws/chat/${channelUsername}`,
			);
			socketRef.current = socket;

			socket.onopen = () => setConnected(true);
			socket.onclose = () => {
				setConnected(false);
				if (!cancelled) {
					retryTimeout = setTimeout(connect, RECONNECT_DELAY_MS);
				}
			};
			socket.onmessage = (event) => {
				let data: IncomingMessage;
				try {
					data = JSON.parse(event.data);
				} catch {
					return;
				}
				if (data.type === "chat") {
					setMessages((prev) => [
						...prev.slice(-(MAX_MESSAGES - 1)),
						{
							id: crypto.randomUUID(),
							username: data.username,
							text: data.message,
						},
					]);
				} else if (data.type === "error") {
					setSendError(data.error);
				} else if (data.type === "viewer_count") {
					setLiveViewerCount(data.viewer_count);
				}
			};
		}

		connect();

		return () => {
			cancelled = true;
			clearTimeout(retryTimeout);
			socketRef.current?.close();
		};
	}, [channelUsername]);

	const sendMessage = useCallback((text: string) => {
		if (socketRef.current?.readyState === WebSocket.OPEN) {
			socketRef.current.send(JSON.stringify({ message: text }));
		}
	}, []);

	return { messages, connected, sendError, sendMessage, liveViewerCount };
}
