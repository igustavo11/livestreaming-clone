import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import { DashboardChannelResponseSchema } from "@/shared/schemas/channel";

export function useUploadAvatar() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (file: File) => {
			const form = new FormData();
			form.append("avatar", file);
			const data = await apiClient.postForm("/api/me/channel/avatar", form);
			return DashboardChannelResponseSchema.parse(data).channel;
		},
		onSuccess: (channel) => {
			queryClient.setQueryData(["me", "channel"], channel);
		},
	});
}
