import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";

type ChannelAvatarProps = {
	username: string;
	avatarUrl: string;
	className?: string;
};

export function ChannelAvatar({
	username,
	avatarUrl,
	className,
}: ChannelAvatarProps) {
	return (
		<Avatar className={cn(className)}>
			{avatarUrl ? <AvatarImage src={avatarUrl} alt="" /> : null}
			<AvatarFallback>{username.slice(0, 2).toUpperCase()}</AvatarFallback>
		</Avatar>
	);
}
