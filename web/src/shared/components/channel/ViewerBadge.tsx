import { cn } from "@/lib/utils";

function formatViewerCount(count: number): string {
	if (count >= 1000) return `${(count / 1000).toFixed(1)}K`;
	return String(count);
}

export function ViewerBadge({
	count,
	className,
}: {
	count: number;
	className?: string;
}) {
	return (
		<span
			className={cn(
				"pointer-events-none rounded-[5px] bg-black/60 px-2 py-0.5 text-[11px] font-semibold text-foreground",
				className,
			)}
		>
			{formatViewerCount(count)}
		</span>
	);
}
