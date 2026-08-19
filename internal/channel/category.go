package channel

// Fixed channel categories (API slugs). Labels may be localized in the UI.
var validCategories = map[string]struct{}{
	"gaming":        {},
	"just_chatting": {},
	"music":         {},
	"irl":           {},
	"esports":       {},
	"art":           {},
	"education":     {},
	"tech":          {},
	"cooking":       {},
	"other":         {},
}

// ValidCategory reports whether slug is an allowed category.
func ValidCategory(slug string) bool {
	_, ok := validCategories[slug]
	return ok
}

// Categories returns the fixed set of category slugs in stable order.
func Categories() []string {
	return []string{
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
	}
}
