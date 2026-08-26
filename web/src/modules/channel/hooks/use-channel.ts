import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import { PublicChannelResponseSchema } from "@/shared/schemas/channel";

export function useChannel(username: string) {
	return useQuery({
		queryKey: ["channel", username],
		queryFn: async () => {
			const data = await apiClient.get(
				`/api/channels/${encodeURIComponent(username)}`,
			);
			return PublicChannelResponseSchema.parse(data).channel;
		},
	});
}
