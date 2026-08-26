import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { useFollow } from "@/modules/channel/hooks/use-follow";
import { ApiError } from "@/shared/lib/api-client";

type FollowButtonProps = {
	username: string;
	isFollowing: boolean;
};

export function FollowButton({ username, isFollowing }: FollowButtonProps) {
	const follow = useFollow(username);

	return (
		<Button
			variant={isFollowing ? "outline" : "default"}
			disabled={follow.isPending}
			onClick={() =>
				follow.mutate(!isFollowing, {
					onError: (err) => {
						toast.error(
							err instanceof ApiError
								? err.message
								: "Não foi possível atualizar",
						);
					},
				})
			}
		>
			{isFollowing ? "Seguindo" : "Seguir"}
		</Button>
	);
}
