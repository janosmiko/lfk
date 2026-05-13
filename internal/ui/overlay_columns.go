package ui

// ColumnToggleEntry is the UI-facing column toggle entry. Constructed by the
// caller from the model's column-visibility state and passed into the
// OverlayList-based column toggle helper (see internal/app).
type ColumnToggleEntry struct {
	Key     string
	Visible bool
}
