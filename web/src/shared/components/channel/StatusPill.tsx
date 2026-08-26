import { cn } from "@/lib/utils";

export function StatusPill({
	live,
	className,
}: {
	live: boolean;
	className?: string;
}) {
	return (
		<span
			className={cn(
				"inline-flex items-center gap-1.5 rounded-full bg-white/[0.06] px-3 py-1.5 text-xs font-semibold text-foreground/70",
				className,
			)}
		>
			<span
				className={cn(
					"size-2 rounded-full",
					live ? "bg-primary" : "bg-[#6B6B6B]",
				)}
			/>
			{live ? "Ao vivo" : "Offline"}
		</span>
	);
}
