import { cn } from "@/lib/utils";

export function LiveBadge({ className }: { className?: string }) {
	return (
		<span
			className={cn(
				"pointer-events-none inline-flex items-center rounded-[5px] bg-primary px-2 py-0.5 text-[11px] font-bold tracking-wide text-primary-foreground",
				className,
			)}
		>
			AO VIVO
		</span>
	);
}
