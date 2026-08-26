import { Navigate, useParams } from "react-router-dom";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { ChatPanel } from "@/modules/channel/components/ChatPanel";
import { FollowButton } from "@/modules/channel/components/FollowButton";
import { PlayerEmbed } from "@/modules/channel/components/PlayerEmbed";
import { useChannel } from "@/modules/channel/hooks/use-channel";
import { useChatSocket } from "@/modules/channel/hooks/use-chat-socket";
import { ChannelAvatar } from "@/shared/components/channel/ChannelAvatar";
import { LiveBadge } from "@/shared/components/channel/LiveBadge";
import { AppHeader } from "@/shared/components/layout/AppHeader";
import { useAuth } from "@/shared/hooks/use-auth";
import { ApiError } from "@/shared/lib/api-client";
import { categoryLabel } from "@/shared/lib/categories";

function formatFollowerCount(count: number): string {
	if (count === 1) return "1 seguidor";
	return `${count} seguidores`;
}

export function ChannelPage() {
	const { username = "" } = useParams<{ username: string }>();
	const channel = useChannel(username);
	const chat = useChatSocket(username);
	const { session } = useAuth();

	if (
		channel.isError &&
		channel.error instanceof ApiError &&
		channel.error.status === 404
	) {
		return <Navigate to="/" replace />;
	}

	const isOwnChannel =
		session !== null && session.channel.username === username;

	return (
		<div className="flex min-h-svh flex-col bg-background">
			<AppHeader />

			<div className="flex flex-1 flex-col md:min-h-0 md:flex-row">
				<div className="flex min-w-0 flex-col md:flex-1">
					{channel.data ? (
						<PlayerEmbed username={channel.data.username} />
					) : (
						<Skeleton className="aspect-video w-full rounded-none" />
					)}

					<div className="flex items-start justify-between gap-6 border-b border-border px-6 py-5 md:px-8">
						{channel.data ? (
							<>
								<div className="flex gap-3.5">
									<ChannelAvatar
										username={channel.data.username}
										avatarUrl={channel.data.avatar_url}
										className="size-12 shrink-0"
									/>
									<div className="flex flex-col gap-1">
										<span className="text-lg font-bold">
											{channel.data.username}
										</span>
										<span className="text-sm text-foreground/75">
											{channel.data.title || "Sem título"}
										</span>
										<div className="mt-0.5 flex items-center gap-2.5">
											<span className="text-xs text-muted-foreground">
												{chat.liveViewerCount ?? channel.data.viewer_count}{" "}
												espectadores
											</span>
											<span className="text-xs text-muted-foreground">
												{formatFollowerCount(channel.data.follower_count)}
											</span>
											<Badge variant="secondary" className="rounded-full">
												{categoryLabel(channel.data.category)}
											</Badge>
											{channel.data.is_live ? <LiveBadge /> : null}
										</div>
									</div>
								</div>
								{session && !isOwnChannel ? (
									<FollowButton
										username={channel.data.username}
										isFollowing={channel.data.is_following}
									/>
								) : null}
							</>
						) : (
							<Skeleton className="h-12 w-64" />
						)}
					</div>
				</div>

				<ChatPanel
					messages={chat.messages}
					canSend={session !== null}
					onSend={chat.sendMessage}
				/>
			</div>
		</div>
	);
}
