import { useMutation, useQueryClient } from "@tanstack/react-query";
import type { UpdateChannelInput } from "@/modules/dashboard/lib/schemas";
import { apiClient } from "@/shared/lib/api-client";
import { DashboardChannelResponseSchema } from "@/shared/schemas/channel";

export function useUpdateChannel() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (input: UpdateChannelInput) => {
			const data = await apiClient.put("/api/me/channel", input);
			return DashboardChannelResponseSchema.parse(data).channel;
		},
		onSuccess: (channel) => {
			queryClient.setQueryData(["me", "channel"], channel);
		},
	});
}
