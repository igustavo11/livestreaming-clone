import { useInfiniteQuery } from "@tanstack/react-query";
import { apiClient } from "@/shared/lib/api-client";
import { ChannelListResponseSchema } from "@/shared/schemas/channel";

export const CHANNELS_PAGE_SIZE = 24;

export function useChannels(category: string | null, search: string) {
	return useInfiniteQuery({
		queryKey: ["channels", category, search],
		queryFn: async ({ pageParam }) => {
			const params = new URLSearchParams({
				limit: String(CHANNELS_PAGE_SIZE),
				offset: String(pageParam),
			});
			if (category) params.set("category", category);
			if (search) params.set("search", search);
			const data = await apiClient.get(`/api/channels?${params.toString()}`);
			return ChannelListResponseSchema.parse(data);
		},
		initialPageParam: 0,
		getNextPageParam: (lastPage, allPages) =>
			lastPage.has_more ? allPages.length * CHANNELS_PAGE_SIZE : undefined,
	});
}
