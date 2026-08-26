import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import { DashboardChannelResponseSchema } from "@/shared/schemas/channel";

export function useUploadThumbnail() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async (file: File) => {
			const form = new FormData();
			form.append("thumbnail", file);
			const data = await apiClient.postForm("/api/me/channel/thumbnail", form);
			return DashboardChannelResponseSchema.parse(data).channel;
		},
		onSuccess: (channel) => {
			queryClient.setQueryData(["me", "channel"], channel);
		},
	});
}
