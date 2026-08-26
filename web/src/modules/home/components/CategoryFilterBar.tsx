import type { ReactNode } from "react";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { CATEGORY_LABELS, CATEGORY_SLUGS } from "@/shared/lib/categories";

type CategoryFilterBarProps = {
	value: string | null;
	onChange: (category: string | null) => void;
};

export function CategoryFilterBar({ value, onChange }: CategoryFilterBarProps) {
	return (
		<div className="no-scrollbar flex gap-2.5 overflow-x-auto scroll-smooth px-6 pt-6 pb-2 md:px-10">
			<Pill active={value === null} onClick={() => onChange(null)}>
				Todos
			</Pill>
			{CATEGORY_SLUGS.map((slug) => (
				<Pill key={slug} active={value === slug} onClick={() => onChange(slug)}>
					{CATEGORY_LABELS[slug]}
				</Pill>
			))}
		</div>
	);
}

function Pill({
	active,
	onClick,
	children,
}: {
	active: boolean;
	onClick: () => void;
	children: ReactNode;
}) {
	return (
		<Button
			type="button"
			variant="ghost"
			onClick={onClick}
			className={cn(
				"h-auto shrink-0 whitespace-nowrap rounded-full px-4.5 py-2 text-[13px] font-semibold transition-colors",
				active
					? "bg-primary text-primary-foreground hover:bg-primary-hover hover:text-primary-foreground"
					: "bg-white/[0.06] text-foreground/70 hover:bg-white/[0.1] hover:text-foreground/70",
			)}
		>
			{children}
		</Button>
	);
}
