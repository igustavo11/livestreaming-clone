import { ChevronLeft, ChevronRight } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import { ChatInput } from "@/modules/channel/components/ChatInput";
import type { ChatMessage } from "@/modules/channel/hooks/use-chat-socket";

const USER_COLORS = ["#927a87", "#7fd6c2", "#e8a3a3", "#8fb8e8", "#c6a3e8"];

function colorForUsername(username: string): string {
	let hash = 0;
	for (const char of username)
		hash = (hash * 31 + char.charCodeAt(0)) % USER_COLORS.length;
	return USER_COLORS[hash];
}

type ChatPanelProps = {
	messages: ChatMessage[];
	canSend: boolean;
	onSend: (message: string) => void;
};

export function ChatPanel({ messages, canSend, onSend }: ChatPanelProps) {
	const bottomRef = useRef<HTMLDivElement>(null);
	// Minimize is a desktop-only affordance (theater-mode style) — on mobile
	// the chat is already a stacked section, not a sidebar worth reclaiming.
	const [collapsed, setCollapsed] = useState(false);

	useEffect(() => {
		bottomRef.current?.scrollIntoView({ block: "end" });
	}, []);

	if (collapsed) {
		return (
			<div className="hidden shrink-0 border-l border-border md:flex md:flex-col md:items-center md:pt-2">
				<Button
					variant="ghost"
					size="icon"
					aria-label="Abrir chat"
					onClick={() => setCollapsed(false)}
				>
					<ChevronLeft />
				</Button>
			</div>
		);
	}

	return (
		<aside className="flex w-full shrink-0 flex-col border-border md:w-[340px] md:border-l">
			<div className="flex items-center justify-between border-b border-border px-4.5 py-3.5">
				<span className="text-[13px] font-semibold text-muted-foreground">
					Chat
				</span>
				<Button
					variant="ghost"
					size="icon-sm"
					aria-label="Minimizar chat"
					className="hidden md:inline-flex"
					onClick={() => setCollapsed(true)}
				>
					<ChevronRight />
				</Button>
			</div>
			<ScrollArea className="h-64 flex-1 px-4.5 py-3.5 md:h-auto">
				<div className="flex flex-col gap-3">
					{messages.map((message) => (
						<p key={message.id} className="text-[13px] leading-relaxed">
							<span
								className="font-semibold"
								style={{ color: colorForUsername(message.username) }}
							>
								{message.username}
							</span>{" "}
							<span className="text-foreground/85">{message.text}</span>
						</p>
					))}
					<div ref={bottomRef} />
				</div>
			</ScrollArea>
			<div className="border-t border-border px-4.5 py-3.5">
				<ChatInput disabled={!canSend} onSend={onSend} />
			</div>
		</aside>
	);
}
