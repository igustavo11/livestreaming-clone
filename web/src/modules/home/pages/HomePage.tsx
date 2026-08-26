import { useMemo, useState } from "react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { CategoryFilterBar } from "@/modules/home/components/CategoryFilterBar";
import { useChannels } from "@/modules/home/hooks/use-channels";
import { ChannelCard } from "@/shared/components/channel/ChannelCard";
import { AppHeader } from "@/shared/components/layout/AppHeader";
import { useDebouncedValue } from "@/shared/hooks/use-debounced-value";

export function HomePage() {
	const [category, setCategory] = useState<string | null>(null);
	const [search, setSearch] = useState("");
	const debouncedSearch = useDebouncedValue(search, 400);
	const channels = useChannels(category, debouncedSearch);

	const visibleChannels = useMemo(
		() => channels.data?.pages.flatMap((page) => page.channels) ?? [],
		[channels.data],
	);

	const searchInput = (
		<Input
			value={search}
			onChange={(event) => setSearch(event.target.value)}
			placeholder="Buscar canais ou categorias"
			className="h-9 w-full md:max-w-80"
		/>
	);

	return (
		<div className="min-h-svh bg-background">
			<AppHeader center={searchInput} />

			{/* Mobile mockup puts search on its own full-width row below the header
			    instead of inline — AppHeader's center slot only shows from md up. */}
			<div className="px-6 pt-3 md:hidden">{searchInput}</div>

			<CategoryFilterBar value={category} onChange={setCategory} />

			<div className="grid grid-cols-1 gap-x-6 gap-y-7 px-6 pt-2 pb-10 sm:grid-cols-2 md:px-10 lg:grid-cols-3 xl:grid-cols-4">
				{channels.isLoading
					? Array.from({ length: 8 }).map((_, index) => (
							// biome-ignore lint/suspicious/noArrayIndexKey: static placeholder count, no real identity
							<ChannelCardSkeleton key={index} />
						))
					: visibleChannels.map((channel) => (
							<ChannelCard key={channel.id} channel={channel} />
						))}
			</div>

			{!channels.isLoading && visibleChannels.length === 0 ? (
				<p className="px-6 pb-16 text-sm text-muted-foreground md:px-10">
					Nenhum canal ao vivo no momento.
				</p>
			) : null}

			{channels.hasNextPage ? (
				<div className="flex justify-center pb-16">
					<Button
						variant="outline"
						onClick={() => channels.fetchNextPage()}
						disabled={channels.isFetchingNextPage}
					>
						{channels.isFetchingNextPage ? "Carregando..." : "Carregar mais"}
					</Button>
				</div>
			) : null}
		</div>
	);
}

function ChannelCardSkeleton() {
	return (
		<div className="flex flex-col gap-3">
			<Skeleton className="aspect-video w-full rounded-xl" />
			<div className="flex gap-2.5">
				<Skeleton className="size-9 shrink-0 rounded-full" />
				<div className="flex flex-1 flex-col gap-2">
					<Skeleton className="h-3.5 w-2/3" />
					<Skeleton className="h-3 w-1/3" />
				</div>
			</div>
		</div>
	);
}
