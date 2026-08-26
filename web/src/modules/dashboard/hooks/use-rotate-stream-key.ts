import { useMutation, useQueryClient } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import { RotateStreamKeyResponseSchema } from "@/shared/schemas/channel";

export function useRotateStreamKey() {
	const queryClient = useQueryClient();

	return useMutation({
		mutationFn: async () => {
			const data = await apiClient.post("/api/me/channel/stream-key");
			return RotateStreamKeyResponseSchema.parse(data);
		},
		onSuccess: ({ channel }) => {
			queryClient.setQueryData(["me", "channel"], channel);
		},
	});
}
