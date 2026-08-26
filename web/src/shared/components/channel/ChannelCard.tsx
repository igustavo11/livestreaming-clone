import { Link } from "react-router-dom";
import { ChannelAvatar } from "@/shared/components/channel/ChannelAvatar";
import { LiveBadge } from "@/shared/components/channel/LiveBadge";
import { ViewerBadge } from "@/shared/components/channel/ViewerBadge";
import { categoryLabel } from "@/shared/lib/categories";
import type { PublicChannel } from "@/shared/schemas/channel";

export function ChannelCard({ channel }: { channel: PublicChannel }) {
	return (
		<Link to={`/${channel.username}`} className="flex flex-col gap-3">
			<div className="relative aspect-video overflow-hidden rounded-xl bg-secondary">
				{channel.thumbnail_url ? (
					<img
						src={channel.thumbnail_url}
						alt=""
						className="size-full object-cover"
					/>
				) : null}
				{channel.is_live ? (
					<LiveBadge className="absolute top-2.5 left-2.5" />
				) : null}
				<ViewerBadge
					count={channel.viewer_count}
					className="absolute right-2.5 bottom-2.5"
				/>
			</div>
			<div className="flex items-start gap-2.5">
				<ChannelAvatar
					username={channel.username}
					avatarUrl={channel.avatar_url}
				/>
				<div className="flex min-w-0 flex-col gap-0.5">
					<span className="truncate text-sm font-semibold">
						{channel.username}
					</span>
					<span className="truncate text-xs text-muted-foreground">
						{categoryLabel(channel.category)}
					</span>
				</div>
			</div>
		</Link>
	);
}
