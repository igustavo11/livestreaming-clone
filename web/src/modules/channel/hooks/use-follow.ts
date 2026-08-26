import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import type { PublicChannel } from "@/shared/schemas/channel";
import { FollowResponseSchema } from "@/shared/schemas/channel";

export function useFollow(username: string) {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (follow: boolean) => {
			const data = follow
				? await apiClient.post(
						`/api/channels/${encodeURIComponent(username)}/follow`,
					)
				: await apiClient.delete(
						`/api/channels/${encodeURIComponent(username)}/follow`,
					);
			return FollowResponseSchema.parse(data);
		},
		onSuccess: (result) => {
			queryClient.setQueryData<PublicChannel>(
				["channel", username],
				(current) =>
					current
						? {
								...current,
								follower_count: result.follower_count,
								is_following: result.is_following,
							}
						: current,
			);
		},
	});
}
