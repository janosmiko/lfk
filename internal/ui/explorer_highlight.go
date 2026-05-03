package ui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// MiddleColumnRegion records the byte range a single column occupies in the
// header row of the most recently rendered middle-column table. Key refers
// to a built-in key (Namespace/Ready/Restarts/Status/Age), "Name", or an
// extra column key. StartX is inclusive, EndX is exclusive.
type MiddleColumnRegion struct {
	Key    string
	StartX int
	EndX   int
}

type TableLayoutCache struct {
	Computed bool

	HasNs, HasReady, HasRestarts, HasAge, HasStatus bool
	NsW, ReadyW, RestartsW, AgeW, StatusW           int
	AnyRecentRestart                                bool
	ExtraCols                                       []extraColumn
}

// ActiveSelectedStyle returns SelectedStyle or a nyan rainbow style if nyan
// mode is active. In no-color mode the nyan rainbow is suppressed (colors
// would be stripped anyway) and SelectedStyle is used for visibility.
func ActiveSelectedStyle(rowIdx int) lipgloss.Style {
	if !NyanMode || ConfigNoColor {
		return SelectedStyle
	}
	bgColor := nyanPalette[(NyanTick+rowIdx)%len(nyanPalette)]
	return lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color(bgColor))
}

// VimScrollOff computes the viewport start position using vim-style scrolloff.
// It takes the current scroll position and adjusts it only when the cursor
// would be outside the visible area or within the scrolloff margin.
// displayLines(from, to) returns the number of display lines for entries [from, to).
func VimScrollOff(scroll, cursor, numEntries, height, scrollOff int, displayLines func(from, to int) int) int {
	if cursor < 0 || numEntries <= 0 {
		return 0
	}
	total := displayLines(0, numEntries)
	if total <= height {
		return 0
	}
	if maxSO := (height - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}

	startEntry := max(scroll, 0)
	if startEntry >= numEntries {
		startEntry = numEntries - 1
	}

	// Ensure cursor is visible: scroll down if cursor is below viewport.
	for startEntry < numEntries {
		dl := displayLines(startEntry, cursor+1)
		if dl <= height {
			break
		}
		startEntry++
	}

	// Ensure cursor is visible: scroll up if cursor is above viewport.
	if cursor < startEntry {
		startEntry = cursor
	}

	// Bottom scrolloff: ensure entries after cursor up to scrollOff fit in viewport.
	bottomTarget := min(cursor+scrollOff, numEntries-1)
	for startEntry < numEntries-1 {
		dl := displayLines(startEntry, bottomTarget+1)
		if dl <= height {
			break
		}
		startEntry++
	}

	// Top scrolloff: ensure cursor is at least scrollOff entries from the top.
	topTarget := max(cursor-scrollOff, 0)
	if startEntry > topTarget {
		startEntry = topTarget
	}

	// Don't leave empty space at the bottom — shift the viewport
	// UP while the resulting position still fits. Check the new
	// position BEFORE committing: if decrementing would push the
	// total past height (common when the previous entry has 2-3
	// display lines — a category header with its blank separator
	// and item), stop. Otherwise the viewport ends up at a start
	// that over-runs the bottom and the last 1-2 items get clipped.
	for startEntry > 0 {
		if displayLines(startEntry-1, numEntries) > height {
			break
		}
		startEntry--
	}

	if startEntry < 0 {
		startEntry = 0
	}

	return startEntry
}

// resolveIcon returns the glyph for the active IconMode, or empty string for
// "none" and zero-value icons. Unknown IconMode values fall back to Unicode.
func resolveIcon(icon model.Icon) string {
	if icon.IsEmpty() {
		return ""
	}
	switch IconMode {
	case "none":
		return ""
	case "nerdfont":
		return icon.NerdFont
	case "simple":
		return icon.Simple
	case "emoji":
		return icon.Emoji
	default: // "unicode" and any unexpected value
		return icon.Unicode
	}
}

// isItemSelected checks if an item is in the active selection set.
func isItemSelected(item model.Item) bool {
	if ActiveSelectedItems == nil {
		return false
	}
	key := item.Name
	if item.Namespace != "" {
		key = item.Namespace + "/" + item.Name
	}
	return ActiveSelectedItems[key]
}

// highlightName highlights matched portions of query in name using SearchHighlightStyle.
// Supports substring, regex, and fuzzy search modes.
func highlightName(name, query string) string {
	return HighlightMatchStyled(name, query, SearchHighlightStyle)
}

// highlightNameOver behaves like highlightName but re-asserts
// outerStyle's open codes after each match's reset, so the
// surrounding category-bar / cursor-row background isn't wiped out
// for the post-match part of the line.
func highlightNameOver(name, query string, outerStyle lipgloss.Style) string {
	return HighlightMatchStyledOver(name, query, SearchHighlightStyle, outerStyle)
}

// highlightNameSelected highlights matched portions of query in name
// using SelectedSearchHighlightStyle (for items under the cursor).
func highlightNameSelected(name, query string) string {
	return HighlightMatchStyled(name, query, SelectedSearchHighlightStyle)
}

// highlightNameSelectedOver behaves like highlightNameSelected but
// re-asserts outerStyle's open codes after each match's reset.
func highlightNameSelectedOver(name, query string, outerStyle lipgloss.Style) string {
	return HighlightMatchStyledOver(name, query, SelectedSearchHighlightStyle, outerStyle)
}

// readOnlyPrefix returns the "[RO] " prefix for read-only context rows,
// styled with ReadOnlyMarkerStyle (foreground-only, same visual weight as
// the "* " current-context marker). Empty string when item is not
// read-only so callers can always concatenate. The loud
// ReadOnlyBadgeStyle is reserved for the title-bar header where it
// indicates the active session's state.
func readOnlyPrefix(item model.Item) string {
	if !item.ReadOnly {
		return ""
	}
	return ReadOnlyMarkerStyle.Render("[RO]") + " "
}

// readOnlyPrefixPlain returns the "[RO] " prefix without ANSI styling. Used
// by FormatItemPlain and FormatItemNameOnlyPlain (selected/highlighted rows)
// so the selection background renders cleanly over the prefix instead of
// being interrupted by a nested ANSI reset.
func readOnlyPrefixPlain(item model.Item) string {
	if !item.ReadOnly {
		return ""
	}
	return "[RO] "
}
