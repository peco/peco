package config

// LayoutType describes the types of layout that peco can take
type LayoutType = string

const (
	DefaultLayoutType            = LayoutTypeTopDown       // LayoutTypeTopDown makes the layout so the items read from top to bottom
	LayoutTypeTopDown            = "top-down"              // LayoutTypeTopDown displays prompt at top, list top-to-bottom
	LayoutTypeBottomUp           = "bottom-up"             // LayoutTypeBottomUp displays prompt at bottom, list bottom-to-top
	LayoutTypeTopDownQueryBottom = "top-down-query-bottom" // LayoutTypeTopDownQueryBottom displays list top-to-bottom, prompt at bottom
)

// validLayoutTypes enumerates all recognized layout type values.
var validLayoutTypes = map[LayoutType]struct{}{
	LayoutTypeTopDown:            {},
	LayoutTypeBottomUp:           {},
	LayoutTypeTopDownQueryBottom: {},
}

// IsValidLayoutType checks if a string is a supported layout type
func IsValidLayoutType(v LayoutType) bool {
	_, ok := validLayoutTypes[v]
	return ok
}
