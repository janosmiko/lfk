package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// ActiveHighlightQuery is set by the app to highlight matching text in item names.
var ActiveHighlightQuery string

// ActiveFullscreenMode is set by the app to indicate fullscreen middle column mode.
// In fullscreen mode, more columns (IP, Node, etc.) are shown.
var ActiveFullscreenMode bool

// ActiveContext is set by the app to the current cluster context name.
// Used by collectExtraColumns for per-cluster column config lookups.
var ActiveContext string

// ActiveExtraColumnKeys holds the keys of the extra columns currently displayed
// in the middle column table. Set during RenderTable for the column toggle overlay.
var ActiveExtraColumnKeys []string

// ActiveCollapsedCategories is set by the app before rendering the resource types
// column. Keys are category names; presence means the category is collapsed.
var ActiveCollapsedCategories map[string]bool

// ActiveMiddleScroll is the persistent scroll position for the middle column.
// The render functions use it as the starting scroll position and apply
// vim-style scrolloff logic instead of recalculating from scratch each frame.
// A value of -1 means "no persistent scroll, calculate from scratch".
var ActiveMiddleScroll int

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

	startEntry := scroll
	if startEntry < 0 {
		startEntry = 0
	}
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

	// Bottom scrolloff: if cursor+scrollOff < numEntries, ensure those entries fit.
	if cursor+scrollOff < numEntries {
		for startEntry < numEntries-1 {
			dl := displayLines(startEntry, cursor+scrollOff+1)
			if dl <= height {
				break
			}
			startEntry++
		}
	}

	// Top scrolloff: ensure cursor is at least scrollOff entries from the top.
	topTarget := cursor - scrollOff
	if topTarget < 0 {
		topTarget = 0
	}
	if startEntry > topTarget {
		startEntry = topTarget
	}

	// Don't leave empty space at the bottom.
	for startEntry > 0 && displayLines(startEntry, numEntries) < height {
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

// simpleIcons maps unicode icons to short ASCII labels for "simple" icon mode.
var simpleIcons = map[string]string{
	"⬤": "[Po]",
	"◆": "[De]",
	"◈": "[RS]",
	"◇": "[SS]",
	"●": "[DS]",
	"▶": "[Jo]",
	"⟳": "[CJ]",
	"≡": "[CM]",
	"⊡": "[Se]",
	"⇔": "[HP]",
	"⊞": "[PV]",
	"⊟": "[LR]",
	"⊘": "[PD]",
	"⇑": "[PC]",
	"⊙": "[RC]",
	"⏱": "[Le]",
	"⚙": "[WH]",
	"⇌": "[Sv]",
	"↳": "[In]",
	"⛊": "[NP]",
	"⇢": "[EP]",
	"⇶": "[GW]",
	"▤": "[SC]",
	"▣": "[NS]",
	"⎈": "[Ar]",
	"⎋": "[He]",
	"⊕": "[SA]",
	"⚿": "[Ro]",
	"⬡": "[No]",
	"⧫": "[CR]",
	"↯": "[Ev]",
	"◎": "[OV]",
	"⇋": "[PF]",
}

// emojiIcons maps unicode icons to emoji for "emoji" icon mode.
var emojiIcons = map[string]string{
	"⬤": "🔵",  // Pods
	"◆": "🚀",  // Deployments
	"◈": "📦",  // ReplicaSets
	"◇": "🗄️", // StatefulSets
	"●": "🔄",  // DaemonSets
	"▶": "⚡️", // Jobs
	"⟳": "⏰️", // CronJobs
	"≡": "📋",  // ConfigMaps
	"⊡": "🔒",  // Secrets
	"⇔": "📊",  // HPA
	"⊞": "📏",  // ResourceQuotas / PVCs / PVs
	"⊟": "📐",  // LimitRanges
	"⇕": "📈",  // VPA
	"⊘": "🛡️", // PodDisruptionBudgets
	"⇌": "🔀",  // Services
	"↳": "🌐",  // Ingresses / IngressClasses
	"⛊": "🔥",  // NetworkPolicies
	"⇢": "📍",  // Endpoints / EndpointSlices
	"⇶": "🚪",  // Gateway API resources
	"▤": "💾",  // StorageClasses
	"⊕": "👤",  // ServiceAccounts
	"⚿": "🔑",  // Roles / RoleBindings / ClusterRoles / ClusterRoleBindings
	"▣": "📦",  // Namespaces
	"↯": "🔔",  // Events
	"⬡": "🖥️", // Nodes
	"⧫": "🔷",  // CRDs / API Services
	"⎋": "⛵",  // Helm
	"⎈": "☸️", // ArgoCD
	"⇑": "🏷️", // PriorityClasses
	"⊙": "⚙️", // RuntimeClasses
	"⏱": "⏱️", // Leases
	"⚙": "🔧",  // Webhook Configurations
	"◎": "🏠",  // Cluster Dashboard
	"⇋": "🔗",  // Port Forwards
}

// resolveIcon returns the appropriate icon string based on the current IconMode.
// In "unicode" mode, icons are returned as-is. In "simple" mode, they are mapped
// to short ASCII labels. In "emoji" mode, they are mapped to emoji. In "none" mode,
// an empty string is returned.
func resolveIcon(icon string) string {
	if icon == "" {
		return ""
	}
	switch IconMode {
	case "none":
		return ""
	case "simple":
		if simple, ok := simpleIcons[icon]; ok {
			return simple
		}
		return "[?]"
	case "emoji":
		if e, ok := emojiIcons[icon]; ok {
			return e
		}
		return icon
	default: // "unicode"
		return icon
	}
}

// ActiveSelectedItems is set by the app to indicate which items are multi-selected.
// Keys are "namespace/name" or "name" for non-namespaced resources.
var ActiveSelectedItems map[string]bool

// ActiveShowSecretValues controls whether decoded secret values are shown in previews.
var ActiveShowSecretValues bool

// MaskSecretYAML replaces values under `data:` and `stringData:` top-level keys
// in Kubernetes Secret YAML with "********". This prevents leaking secret values
// in YAML previews when secret display is toggled off.
func MaskSecretYAML(yaml string) string {
	lines := strings.Split(yaml, "\n")
	inDataBlock := false
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		trimmed := strings.TrimRight(line, " \t")
		// Detect top-level `data:` or `stringData:` keys (no leading whitespace).
		if trimmed == "data:" || trimmed == "stringData:" {
			inDataBlock = true
			result = append(result, line)
			continue
		}
		// If we're in a data block, check if the line is an indented key-value pair.
		if inDataBlock {
			// A non-empty line without leading whitespace ends the data block.
			if len(trimmed) > 0 && (trimmed[0] != ' ' && trimmed[0] != '\t') {
				inDataBlock = false
				result = append(result, line)
				continue
			}
			// Empty lines within the block are kept as-is.
			if trimmed == "" {
				result = append(result, line)
				continue
			}
			// Indented key: value line -> mask the value.
			if colonIdx := strings.Index(line, ": "); colonIdx > 0 {
				result = append(result, line[:colonIdx]+": \"********\"")
				continue
			}
			// Indented key with colon at end (multiline value) or continuation lines.
			result = append(result, line)
			continue
		}
		result = append(result, line)
	}
	return strings.Join(result, "\n")
}

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

// highlightName highlights all case-insensitive occurrences of query in name
// using SearchHighlightStyle. Returns the original name if query is empty or not found.
func highlightName(name, query string) string {
	if query == "" {
		return name
	}
	lower := strings.ToLower(name)
	queryLower := strings.ToLower(query)
	idx := strings.Index(lower, queryLower)
	if idx < 0 {
		return name
	}
	var result strings.Builder
	for idx >= 0 {
		result.WriteString(name[:idx])
		result.WriteString(SearchHighlightStyle.Render(name[idx : idx+len(query)]))
		name = name[idx+len(query):]
		lower = lower[idx+len(queryLower):]
		idx = strings.Index(lower, queryLower)
	}
	result.WriteString(name)
	return result.String()
}

// highlightNameSelected highlights all case-insensitive occurrences of query in name
// using SelectedSearchHighlightStyle (for items under the cursor).
func highlightNameSelected(name, query string) string {
	if query == "" {
		return name
	}
	lower := strings.ToLower(name)
	queryLower := strings.ToLower(query)
	idx := strings.Index(lower, queryLower)
	if idx < 0 {
		return name
	}
	var result strings.Builder
	for idx >= 0 {
		result.WriteString(name[:idx])
		result.WriteString(SelectedSearchHighlightStyle.Render(name[idx : idx+len(query)]))
		name = name[idx+len(query):]
		lower = lower[idx+len(queryLower):]
		idx = strings.Index(lower, queryLower)
	}
	result.WriteString(name)
	return result.String()
}

// RenderColumn renders a single column with optional header, item list, and cursor highlight.
// The formatItem callback is used to format each item line.
func RenderColumn(header string, items []model.Item, cursor int, width, height int, isActive, loading bool, spinnerView string, errMsg string) string {
	var b strings.Builder

	if header != "" {
		b.WriteString(DimStyle.Bold(true).Render(header))
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

	// Pre-calculate how many display lines each item will consume,
	// accounting for category headers and separator lines.
	type displayEntry struct {
		itemIdx       int
		hasSep        bool // separator line before this category header
		hasHeader     bool // category header line
		isPlaceholder bool // collapsed group placeholder (header-only, no item line)
	}

	entries := make([]displayEntry, 0, len(items))
	lastCategory := ""
	for i, item := range items {
		e := displayEntry{itemIdx: i}
		if item.Category != "" && item.Category != lastCategory {
			lastCategory = item.Category
			if i > 0 {
				e.hasSep = true
			}
			e.hasHeader = true
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
	scrollOff := 10
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
				startEntry = cursor - scrollOff
				if startEntry < 0 {
					startEntry = 0
				}
			}
			for startEntry > 0 && displayLines(startEntry, len(entries)) < height {
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
			first = false
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
			// Highlight category header when cursor is on a collapsed group placeholder.
			if e.isPlaceholder && e.itemIdx == cursor && isActive {
				line := Truncate(headerText, width)
				lineWidth := lipgloss.Width(line)
				if lineWidth < width {
					line += strings.Repeat(" ", width-lineWidth)
				}
				b.WriteString(SelectedStyle.MaxWidth(width).Render(line))
			} else {
				b.WriteString(CategoryStyle.Render(Truncate(headerText, width)))
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
			// Apply search/filter highlight on the selected item with contrasting style.
			if ActiveHighlightQuery != "" {
				line = highlightNameSelected(line, ActiveHighlightQuery)
			}
			// Pad line to full column width for consistent background.
			lineWidth := lipgloss.Width(line)
			if lineWidth < width {
				line += strings.Repeat(" ", width-lineWidth)
			}
			line = SelectedStyle.MaxWidth(width).Render(line)
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

	// Mark current context with a star.
	if item.Status == "current" {
		return Truncate(CurrentMarkerStyle.Render("* ")+name, width)
	}

	// Build detail columns: ready, restarts, age.
	var detailParts []string
	if item.Ready != "" {
		detailParts = append(detailParts, DimStyle.Render(item.Ready))
	}
	if item.Restarts != "" {
		detailParts = append(detailParts, DimStyle.Render(item.Restarts))
	}
	if item.Age != "" {
		detailParts = append(detailParts, AgeStyle(item.Age).Render(item.Age))
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
		maxNameW := width - rightW - 2
		if maxNameW < 8 {
			maxNameW = 8
		}
		visualName := name
		if lipgloss.Width(visualName) > maxNameW {
			rawName := displayName
			iconPrefix := ""
			if icon != "" {
				iconPrefix = IconStyle.Render(icon) + " "
			}
			iconW := lipgloss.Width(iconPrefix)
			available := maxNameW - iconW
			if available < 4 {
				available = 4
			}
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
		padding := width - nameW - rightW - 1
		if padding < 1 {
			padding = 1
		}
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

	// Mark current context with a star (plain text, no CurrentMarkerStyle).
	if item.Status == "current" {
		return Truncate("* "+name, width)
	}

	// Build detail columns: ready, restarts, age.
	var detailParts []string
	if item.Ready != "" {
		detailParts = append(detailParts, item.Ready)
	}
	if item.Restarts != "" {
		detailParts = append(detailParts, item.Restarts)
	}
	if item.Age != "" {
		detailParts = append(detailParts, item.Age)
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
		maxNameW := width - rightW - 2
		if maxNameW < 8 {
			maxNameW = 8
		}
		visualName := name
		if lipgloss.Width(visualName) > maxNameW {
			rawName := displayName
			iconPrefix := ""
			if icon != "" {
				iconPrefix = icon + " "
			}
			iconW := lipgloss.Width(iconPrefix)
			available := maxNameW - iconW
			if available < 4 {
				available = 4
			}
			if len(rawName) > available {
				rawName = rawName[:available-1] + "~"
			}
			visualName = iconPrefix + rawName
		}
		nameW := lipgloss.Width(visualName)
		padding := width - nameW - rightW - 1
		if padding < 1 {
			padding = 1
		}
		return visualName + strings.Repeat(" ", padding) + rightSide
	}

	return Truncate(name, width)
}
