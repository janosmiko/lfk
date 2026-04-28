package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// ActiveHighlightQuery is set by the app to highlight matching text in item names.
var ActiveHighlightQuery string

// ActiveHighlightCategories opts-in to highlighting matching text in
// the category bars too. Only set when the user explicitly opted into
// category-aware search/filter (Tab inside the input at
// LevelResourceTypes); otherwise category bars stay un-highlighted
// even when ActiveHighlightQuery is non-empty, so plain `/foo` doesn't
// flash a category that the user hasn't asked to navigate into.
var ActiveHighlightCategories bool

// ActiveFullscreenMode is set by the app to indicate fullscreen middle column mode.
// In fullscreen mode, more columns (IP, Node, etc.) are shown.
var ActiveFullscreenMode bool

// ActiveContext is set by the app to the current cluster context name.
// Used by collectExtraColumns for per-cluster column config lookups.
var ActiveContext string

// ActiveExtraColumnKeys holds the keys of the extra columns currently displayed
// in the middle column table. Set during RenderTable for the column toggle overlay.
var ActiveExtraColumnKeys []string

// ActiveColumnOrder is the user-specified column order for the current
// middle-column render, excluding the mandatory Name column which is
// always first. Nil means "use the default order". Keys may refer to
// built-in columns (Namespace/Ready/Restarts/Status/Age) or to extra
// column keys from additionalPrinterColumns. Entries whose columns are
// not currently present are silently dropped at render time.
var ActiveColumnOrder []string

// MiddleColumnRegion records the byte range a single column occupies in the
// header row of the most recently rendered middle-column table. Key refers
// to a built-in key (Namespace/Ready/Restarts/Status/Age), "Name", or an
// extra column key. StartX is inclusive, EndX is exclusive.
type MiddleColumnRegion struct {
	Key    string
	StartX int
	EndX   int
}

// ActiveMiddleColumnLayout records the visual layout of the columns rendered
// in the most recent middle-column table. Populated by RenderTable when
// ActiveMiddleScroll >= 0 and consumed by mouse click handling to map a
// click X coordinate to a column key.
var ActiveMiddleColumnLayout []MiddleColumnRegion

// ActiveCollapsedCategories is set by the app before rendering the resource types
// column. Keys are category names; presence means the category is collapsed.
var ActiveCollapsedCategories map[string]bool

// ActiveMiddleScroll is the persistent scroll position for the middle column.
// The render functions use it as the starting scroll position and apply
// vim-style scrolloff logic instead of recalculating from scratch each frame.
// A value of -1 means "no persistent scroll, calculate from scratch".
var ActiveMiddleScroll int

// ActiveRowCache and ActiveTableLayout, when non-nil, let RenderTable reuse
// pre-rendered row strings and pre-computed column widths across calls.
// TableRenderer owns the lifetime — it sets these before each RenderTable
// call and restores them after. Nil disables caching for direct callers.
var ActiveRowCache map[int]string

type TableLayoutCache struct {
	Computed bool

	HasNs, HasReady, HasRestarts, HasAge, HasStatus bool
	NsW, ReadyW, RestartsW, AgeW, StatusW           int
	AnyRecentRestart                                bool
	ExtraCols                                       []extraColumn
}

var ActiveTableLayout *TableLayoutCache

// ActiveLeftScroll is the persistent scroll position for the left column.
// Same semantics as ActiveMiddleScroll.
var ActiveLeftScroll int

// ActiveMiddleLineMap maps display line numbers (0-based, relative to content
// start after the column/table header) to item indices. A value of -1 means
// the line is non-clickable (separator or category header). Built during
// middle column rendering for use by mouse click handling.
var ActiveMiddleLineMap []int

// ActiveSortColumn is the currently sorted column index (visual order).
// Derived from ActiveSortColumnName during RenderTable.
var ActiveSortColumn int

// ActiveSortColumnName is the name of the currently sorted column.
// Set by the app layer before rendering.
var ActiveSortColumnName string

// ActiveSortAscending is true for ascending sort.
var ActiveSortAscending = true

// ActiveSortableColumns holds the names of all sortable columns in visual order.
// Set during RenderTable. Used by the app layer for sort cycling.
var ActiveSortableColumns []string

// ActiveSortableColumnCount is len(ActiveSortableColumns).
var ActiveSortableColumnCount int

// NyanMode enables rainbow cursor styling (easter egg).
var NyanMode bool

// NyanTick is the animation tick for nyan mode rainbow cycling.
var NyanTick int

// nyanPalette is the rainbow color cycle for nyan mode.
var nyanPalette = []string{"#ff0000", "#ff8800", "#ffff00", "#00ff00", "#0088ff", "#8800ff"}

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

// ActiveCategoryCounts is set by the app before rendering the resource types column.
// Maps category name to the total number of items in that category.
var ActiveCategoryCounts map[string]int

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

// ActiveSelectedItems is set by the app to indicate which items are multi-selected.
// Keys are "namespace/name" or "name" for non-namespaced resources.
var ActiveSelectedItems map[string]bool

// ActiveShowSecretValues controls whether secret values are revealed in the
// resource details pane. When false, columns prefixed with `secret:` are
// rendered as `********`. The YAML preview is not affected by this toggle.
var ActiveShowSecretValues bool

// selectionMarker is the unicode checkmark prepended to selected items.
const selectionMarker = "\u2713 "

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

// RenderColumn renders a single column with optional header, item list, and cursor highlight.
// The formatItem callback is used to format each item line.
func RenderColumn(header string, items []model.Item, cursor int, width, height int, isActive, loading bool, spinnerView string, errMsg string) string { //nolint:gocyclo // rendering function with inherent layout complexity
	var b strings.Builder

	if header != "" {
		// Truncate to column width so lipgloss doesn't wrap a long
		// header ("KUBECONFIG", "RESOURCE TYPE") onto a second line.
		// Without truncation, the wrapped header pushes the column 1
		// row taller than its neighbors, the outer view overflows
		// m.height by 1, and the terminal clips the bottom row of the
		// middle column. Truncating here guarantees the header always
		// occupies exactly one rendered line so the height-- below
		// remains correct.
		b.WriteString(DimStyle.Bold(true).Render(Truncate(header, width)))
		b.WriteString("\n")
		height--
	}

	if len(items) == 0 {
		switch {
		case loading:
			b.WriteString(DimStyle.Render(spinnerView+" ") + DimStyle.Render("Loading..."))
		case errMsg != "":
			b.WriteString(ErrorStyle.Render(Truncate(errMsg, width)))
		default:
			b.WriteString(DimStyle.Render("No items"))
		}
		return b.String()
	}

	// Pre-calculate how many display lines each item will consume.
	// Each category starts with a full-width category bar, preceded
	// by a blank separator row for every category header except the
	// very first. When the total exceeds the viewport height, items
	// naturally scroll — VimScrollOff shifts the viewport down to
	// keep the cursor visible, showing the list tail with earlier
	// items (or trailing blank padding) at the top of the window.
	type displayEntry struct {
		itemIdx       int
		hasSep        bool // blank row before this category header
		hasHeader     bool // category header line
		isPlaceholder bool // collapsed group placeholder (header-only, no item line)
	}

	entries := make([]displayEntry, 0, len(items))
	lastCategory := ""
	for i, item := range items {
		e := displayEntry{itemIdx: i}
		if item.Category != "" && item.Category != lastCategory {
			lastCategory = item.Category
			e.hasHeader = true
			if i > 0 {
				e.hasSep = true
			}
		}
		if item.Kind == "__collapsed_group__" {
			e.isPlaceholder = true
		}
		entries = append(entries, e)
	}

	// Calculate the display line count for a range of entries.
	displayLines := func(startEntry, endEntry int) int {
		lines := 0
		for ei := startEntry; ei < endEntry; ei++ {
			e := entries[ei]
			if e.hasSep && ei > startEntry {
				lines++ // separator (not rendered for first visible entry)
			}
			if e.hasHeader {
				lines++
			}
			if !e.isPlaceholder {
				lines++ // the item itself (placeholders have header only)
			}
		}
		return lines
	}

	// Clamp cursor to valid range to prevent panics.
	if cursor >= len(entries) {
		cursor = len(entries) - 1
	}

	// Determine visible window using vim-style scrolloff.
	scrollOff := ConfigScrollOff
	startEntry := 0

	// Use persistent scroll position if available (vim-style stable viewport).
	if isActive && ActiveMiddleScroll >= 0 {
		startEntry = VimScrollOff(ActiveMiddleScroll, cursor, len(entries), height, scrollOff, displayLines)
		ActiveMiddleScroll = startEntry
	} else if !isActive && ActiveLeftScroll >= 0 {
		startEntry = VimScrollOff(ActiveLeftScroll, cursor, len(entries), height, scrollOff, displayLines)
		ActiveLeftScroll = startEntry
	} else {
		// Fallback: calculate from scratch (old behavior for callers that don't set scroll).
		totalDisplayLines := displayLines(0, len(entries))
		if totalDisplayLines <= height {
			scrollOff = 0
		} else if maxSO := (height - 1) / 2; scrollOff > maxSO {
			scrollOff = maxSO
		}
		if cursor >= 0 && totalDisplayLines > height {
			for startEntry < len(entries) {
				dl := displayLines(startEntry, cursor+1)
				if dl <= height {
					break
				}
				startEntry++
			}
			if cursor+scrollOff < len(entries) {
				for startEntry < len(entries) {
					dl := displayLines(startEntry, cursor+scrollOff+1)
					if dl <= height {
						break
					}
					startEntry++
				}
			}
			if cursor-scrollOff >= 0 && startEntry > cursor-scrollOff {
				startEntry = max(cursor-scrollOff, 0)
			}
			// Check the new position BEFORE decrementing so we
			// don't overshoot when an earlier entry has 2-3
			// display lines (header + sep + item).
			for startEntry > 0 {
				if displayLines(startEntry-1, len(entries)) > height {
					break
				}
				startEntry--
			}
		}
	}

	// Find end entry that fits within height.
	endEntry := startEntry
	usedLines := 0
	for endEntry < len(entries) {
		entryLines := 0
		e := entries[endEntry]
		if e.hasSep && endEntry > startEntry {
			entryLines++ // separator (not rendered for first visible entry)
		}
		if e.hasHeader {
			entryLines++
		}
		if !e.isPlaceholder {
			entryLines++ // item line (placeholders have header only)
		}
		if usedLines+entryLines > height {
			break
		}
		usedLines += entryLines
		endEntry++
	}

	// Build display-line-to-item map for mouse click handling (active middle column only).
	if isActive {
		ActiveMiddleLineMap = ActiveMiddleLineMap[:0]
		for ei := startEntry; ei < endEntry; ei++ {
			e := entries[ei]
			if e.hasSep && ei > startEntry {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1)
			}
			if e.hasHeader {
				if e.isPlaceholder {
					ActiveMiddleLineMap = append(ActiveMiddleLineMap, e.itemIdx)
				} else {
					ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1)
				}
			}
			if !e.isPlaceholder {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, e.itemIdx)
			}
		}
	}

	// Render visible entries.
	first := true
	for ei := startEntry; ei < endEntry; ei++ {
		e := entries[ei]
		item := items[e.itemIdx]

		if e.hasSep {
			if !first {
				b.WriteString("\n")
			}
			// Do NOT flip `first` here. The sep row is a visual
			// empty line between groups — when it IS written (not
			// first in viewport) the newline on its own is enough;
			// when it's elided (first visible entry), nothing was
			// drawn, so `first` must remain true so the subsequent
			// header block doesn't prepend a leading "\n" that
			// creates a phantom empty row at the top of the view
			// (and throws off the displayLines() budget by 1).
		}
		if e.hasHeader {
			if !first {
				b.WriteString("\n")
			}
			headerText := item.Category
			if len(ActiveCollapsedCategories) > 0 {
				if ActiveCollapsedCategories[item.Category] {
					// Collapsed: show arrow and item count.
					headerText = fmt.Sprintf("▸ %s (%d)", item.Category, ActiveCategoryCounts[item.Category])
				} else {
					// Expanded: show down arrow.
					headerText = "▾ " + item.Category
				}
			}
			// Highlight matching text in the header only when the
			// user opted into category-aware search/filter via Tab
			// (ActiveHighlightCategories). Without it, plain `/foo`
			// would visibly mark "Argo CD" or "Networking" without
			// actually treating those categories as match targets,
			// confusing the user about what n/N will do.
			//
			// We pass the same outer style that's about to wrap the
			// line so the inner highlight's reset doesn't kill the
			// bar's background for the post-match segment.
			if e.isPlaceholder && e.itemIdx == cursor && isActive {
				highlighted := false
				if ActiveHighlightCategories && ActiveHighlightQuery != "" {
					headerText = highlightNameOver(headerText, ActiveHighlightQuery, ActiveSelectedStyle(e.itemIdx))
					highlighted = true
				}
				line := Truncate(headerText, width)
				lineWidth := lipgloss.Width(line)
				if lineWidth < width {
					line += strings.Repeat(" ", width-lineWidth)
				}
				if highlighted {
					// Use manual outer-wrap: lipgloss.Render fragments
					// embedded inner ANSI per-character, producing
					// malformed sequences that NO_COLOR terminals
					// display as literal "[1;7m..." text and that
					// throw off visible width.
					b.WriteString(RenderOverPrestyled(line, ActiveSelectedStyle(e.itemIdx)))
				} else {
					b.WriteString(ActiveSelectedStyle(e.itemIdx).MaxWidth(width).Render(line))
				}
			} else {
				highlighted := false
				if ActiveHighlightCategories && ActiveHighlightQuery != "" {
					headerText = highlightNameOver(headerText, ActiveHighlightQuery, CategoryBarStyle)
					highlighted = true
				}
				// Render the category header as a full-width "bar"
				// with a distinct style so groups are visually
				// separated without needing a blank row above.
				line := Truncate(headerText, width)
				lineWidth := lipgloss.Width(line)
				if lineWidth < width {
					line += strings.Repeat(" ", width-lineWidth)
				}
				if highlighted {
					b.WriteString(RenderOverPrestyled(line, CategoryBarStyle))
				} else {
					b.WriteString(CategoryBarStyle.Render(line))
				}
			}
			first = false
		}

		// Skip item line for collapsed group placeholders (header-only).
		if e.isPlaceholder {
			continue
		}

		if !first {
			b.WriteString("\n")
		}
		var line string
		switch {
		case e.itemIdx == cursor && isActive:
			line = FormatItemPlain(item, width)
			highlighted := false
			// Apply search/filter highlight on the selected item with
			// contrasting style. Pass the outer cursor style so the
			// inner highlight's reset doesn't kill the cursor bg for
			// the post-match part of the row.
			if ActiveHighlightQuery != "" {
				line = highlightNameSelectedOver(line, ActiveHighlightQuery, ActiveSelectedStyle(e.itemIdx))
				highlighted = true
			}
			// Pad line to full column width for consistent background.
			lineWidth := lipgloss.Width(line)
			if lineWidth < width {
				line += strings.Repeat(" ", width-lineWidth)
			}
			if highlighted {
				// Avoid lipgloss.Render fragmenting embedded inner
				// highlight ANSI — see RenderOverPrestyled doc.
				line = RenderOverPrestyled(line, ActiveSelectedStyle(e.itemIdx))
			} else {
				line = ActiveSelectedStyle(e.itemIdx).MaxWidth(width).Render(line)
			}
		case e.itemIdx == cursor && cursor >= 0:
			// Parent column highlight (dimmer than active selection).
			line = FormatItemNameOnlyPlain(item, width)
			lineWidth := lipgloss.Width(line)
			if lineWidth < width {
				line += strings.Repeat(" ", width-lineWidth)
			}
			line = ParentHighlightStyle.MaxWidth(width).Render(line)
		case !isActive:
			// Inactive columns (parent/child): show name only.
			line = FormatItemNameOnly(item, width)
			line = NormalStyle.Width(width).MaxWidth(width).Render(line)
		default:
			line = FormatItem(item, width)
			line = NormalStyle.Width(width).MaxWidth(width).Render(line)
		}
		b.WriteString(line)
		first = false
	}

	return b.String()
}

// FormatItem formats a single item for display in a column.
func FormatItem(item model.Item, width int) string {
	displayName := item.Name

	// Prepend namespace in all-namespaces mode.
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	name := NormalStyle.Render(displayName)

	// Prepend icon if present (resolved based on IconMode).
	icon := resolveIcon(item.Icon)
	if icon != "" {
		name = IconStyle.Render(icon+" ") + name
	}

	// Append deprecation warning indicator for deprecated API versions.
	if item.Deprecated {
		name += DeprecationStyle.Render(" ⚠")
	}

	// Mark current context with a star and (optionally) prefix the
	// read-only badge. Order: "* [RO] name". Both are styled prefixes so
	// the renderer's ANSI tracking (lipgloss.Width) keeps the visible
	// width correct even when the marker is colored.
	if item.Status == "current" {
		return Truncate(CurrentMarkerStyle.Render("* ")+readOnlyPrefix(item)+name, width)
	}

	// Non-current read-only rows: prepend "[RO] " so the marker is
	// visible regardless of column width.
	if item.ReadOnly {
		return Truncate(readOnlyPrefix(item)+name, width)
	}

	// Build detail columns: ready, restarts, age.
	var detailParts []string
	if item.Ready != "" {
		detailParts = append(detailParts, DimStyle.Render(item.Ready))
	}
	if item.Restarts != "" {
		detailParts = append(detailParts, DimStyle.Render(item.Restarts))
	}
	if age := LiveAge(item); age != "" {
		detailParts = append(detailParts, AgeStyle(age).Render(age))
	}

	// Build the right-side info: details + status.
	var rightSide string
	if len(detailParts) > 0 {
		detailStr := strings.Join(detailParts, " ")
		if item.Status != "" {
			rightSide = detailStr + " " + StatusStyle(item.Status).Render(item.Status)
		} else {
			rightSide = detailStr
		}
	} else if item.Status != "" {
		rightSide = StatusStyle(item.Status).Render(item.Status)
	}

	if rightSide != "" {
		rightW := lipgloss.Width(rightSide)
		maxNameW := max(width-rightW-2, 8)
		visualName := name
		if lipgloss.Width(visualName) > maxNameW {
			rawName := displayName
			iconPrefix := ""
			if icon != "" {
				iconPrefix = IconStyle.Render(icon) + " "
			}
			iconW := lipgloss.Width(iconPrefix)
			available := max(maxNameW-iconW, 4)
			if len(rawName) > available {
				rawName = rawName[:available-1] + "~"
			}
			if ActiveHighlightQuery != "" {
				rawName = highlightName(rawName, ActiveHighlightQuery)
			}
			visualName = iconPrefix + rawName
		} else if ActiveHighlightQuery != "" {
			// Name fits without truncation; apply highlight to displayName portion.
			iconPrefix := ""
			if icon != "" {
				iconPrefix = IconStyle.Render(icon) + " "
			}
			visualName = iconPrefix + highlightName(displayName, ActiveHighlightQuery)
		}
		nameW := lipgloss.Width(visualName)
		padding := max(width-nameW-rightW-1, 1)
		return visualName + strings.Repeat(" ", padding) + rightSide
	}

	// No right side info; apply highlight before truncation for simple case.
	if ActiveHighlightQuery != "" {
		iconPrefix := ""
		if icon != "" {
			iconPrefix = IconStyle.Render(icon) + " "
		}
		highlighted := iconPrefix + highlightName(displayName, ActiveHighlightQuery)
		return Truncate(highlighted, width)
	}
	return Truncate(name, width)
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

// FormatItemPlain formats a single item for display WITHOUT any inner ANSI styling.
// Used for the selected item so the selection background renders cleanly.
func FormatItemPlain(item model.Item, width int) string {
	displayName := item.Name

	// Prepend namespace in all-namespaces mode.
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	name := displayName

	// Prepend icon if present (plain text, no IconStyle; resolved based on IconMode).
	icon := resolveIcon(item.Icon)
	if icon != "" {
		name = icon + " " + name
	}

	// Append deprecation warning indicator (plain text, no styling).
	if item.Deprecated {
		name += " ⚠"
	}

	// Mark current context with a star (plain text, no CurrentMarkerStyle)
	// and prepend a "[RO] " prefix when the row is read-only. Order:
	// "* [RO] name" so the star stays the leftmost glyph and the marker
	// sits next to it like a tag.
	if item.Status == "current" {
		return Truncate("* "+readOnlyPrefixPlain(item)+name, width)
	}

	// Non-current read-only rows: prepend "[RO] " so the marker is
	// visible regardless of column width and the standard right-side
	// info path can still render details / status when applicable.
	if item.ReadOnly {
		// Fall through to the standard formatting with a prefix attached
		// so that any right-side details (currently unused for context
		// rows but possible for other read-only resources in the future)
		// remain consistent.
		return Truncate(readOnlyPrefixPlain(item)+name, width)
	}

	// Build detail columns: ready, restarts, age.
	var detailParts []string
	if item.Ready != "" {
		detailParts = append(detailParts, item.Ready)
	}
	if item.Restarts != "" {
		detailParts = append(detailParts, item.Restarts)
	}
	if age := LiveAge(item); age != "" {
		detailParts = append(detailParts, age)
	}

	// Build the right-side info: details + status (all plain text).
	var rightSide string
	if len(detailParts) > 0 {
		detailStr := strings.Join(detailParts, " ")
		if item.Status != "" {
			rightSide = detailStr + " " + item.Status
		} else {
			rightSide = detailStr
		}
	} else if item.Status != "" {
		rightSide = item.Status
	}

	if rightSide != "" {
		rightW := lipgloss.Width(rightSide)
		maxNameW := max(width-rightW-2, 8)
		visualName := name
		if lipgloss.Width(visualName) > maxNameW {
			rawName := displayName
			iconPrefix := ""
			if icon != "" {
				iconPrefix = icon + " "
			}
			iconW := lipgloss.Width(iconPrefix)
			available := max(maxNameW-iconW, 4)
			if len(rawName) > available {
				rawName = rawName[:available-1] + "~"
			}
			visualName = iconPrefix + rawName
		}
		nameW := lipgloss.Width(visualName)
		padding := max(width-nameW-rightW-1, 1)
		return visualName + strings.Repeat(" ", padding) + rightSide
	}

	return Truncate(name, width)
}
