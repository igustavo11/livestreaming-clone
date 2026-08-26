// Mirrors internal/channel/category.go — the backend is the source of truth
// for valid slugs. There is no "list categories" endpoint, so the set and
// its PT-BR labels are duplicated here for the filter bar and dashboard form.
export const CATEGORY_SLUGS = [
	"gaming",
	"just_chatting",
	"music",
	"irl",
	"esports",
	"art",
	"education",
	"tech",
	"cooking",
	"other",
] as const;

export type CategorySlug = (typeof CATEGORY_SLUGS)[number];

export const CATEGORY_LABELS: Record<CategorySlug, string> = {
	gaming: "Games",
	just_chatting: "Just Chatting",
	music: "Música",
	irl: "IRL",
	esports: "Esports",
	art: "Criativo/Arte",
	education: "Educação",
	tech: "Tecnologia",
	cooking: "Culinária",
	other: "Outros",
};

export function categoryLabel(slug: string): string {
	return CATEGORY_LABELS[slug as CategorySlug] ?? slug;
}
