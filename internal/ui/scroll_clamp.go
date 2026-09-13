package ui

// ClampScroll restricts scroll to the valid range for total rows in a
// viewport of bodyHeight rows.
func ClampScroll(scroll, total, bodyHeight int) int {
	if total <= bodyHeight {
		return 0
	}
	maxScroll := total - bodyHeight
	scroll = max(scroll, 0)
	scroll = min(scroll, maxScroll)
	return scroll
}
