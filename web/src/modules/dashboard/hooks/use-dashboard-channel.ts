import { useQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import { DashboardChannelResponseSchema } from "@/shared/schemas/channel";

export function useDashboardChannel() {
	return useQuery({
		queryKey: ["me", "channel"],
		queryFn: async () => {
			const data = await apiClient.get("/api/me/channel");
			return DashboardChannelResponseSchema.parse(data).channel;
		},
	});
}
