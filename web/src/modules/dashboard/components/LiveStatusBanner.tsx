import { cn } from "@/lib/utils";

export function LiveStatusBanner({ live }: { live: boolean }) {
	return (
		<div className="flex items-center gap-3 rounded-2xl border border-border bg-card px-7 py-6">
			<span
				className={cn(
					"size-2 shrink-0 rounded-full",
					live ? "bg-primary" : "bg-[#6B6B6B]",
				)}
			/>
			<p className="text-sm text-foreground/70">
				Sua live está{" "}
				<span className="font-semibold text-foreground">
					{live ? "ao vivo" : "offline"}
				</span>
				.{" "}
				{live
					? "Espectadores já podem assistir."
					: "Conecte seu software de transmissão com a stream key acima para começar."}
			</p>
		</div>
	);
}
