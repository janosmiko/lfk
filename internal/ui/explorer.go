package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// ActiveHighlightQuery is set by the app to highlight matching text in item names.
var ActiveHighlightQuery string

// ActiveFullscreenMode is set by the app to indicate fullscreen middle column mode.
// In fullscreen mode, more columns (IP, Node, etc.) are shown.
var ActiveFullscreenMode bool

// ActiveCollapsedCategories is set by the app before rendering the resource types
// column. Keys are category names; presence means the category is collapsed.
var ActiveCollapsedCategories map[string]bool

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
	"◇": "🗄️",  // StatefulSets
	"●": "🔄",  // DaemonSets
	"▶": "⚡️",  // Jobs
	"⟳": "⏰️",  // CronJobs
	"≡": "📋",  // ConfigMaps
	"⊡": "🔒",  // Secrets
	"⇔": "📊",  // HPA
	"⊞": "📏",  // ResourceQuotas / PVCs / PVs
	"⊟": "📐",  // LimitRanges
	"⇕": "📈",  // VPA
	"⊘": "🛡️",  // PodDisruptionBudgets
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
	"⬡": "🖥️",  // Nodes
	"⧫": "🔷",  // CRDs / API Services
	"⎋": "⛵",  // Helm
	"⎈": "☸️",  // ArgoCD
	"⇑": "🏷️",  // PriorityClasses
	"⊙": "⚙️",  // RuntimeClasses
	"⏱": "⏱️",  // Leases
	"⚙": "🔧",  // Webhook Configurations
	"◎": "🏠",  // Overview
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
			b.WriteString(DimStyle.Render(spinnerView + " Loading..."))
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
			if e.hasSep {
				lines++
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

	// Determine visible window: scroll so cursor is visible with scrolloff padding.
	scrollOff := 10
	startEntry := 0
	totalDisplayLines := displayLines(0, len(entries))
	// Disable or reduce scrolloff when items fit (or nearly fit) the visible area.
	if totalDisplayLines <= height {
		scrollOff = 0
	} else if maxSO := (height - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}
	if cursor >= 0 && totalDisplayLines > height {
		// Only apply scrolling if content doesn't fit on screen.
		// First, ensure the cursor is visible.
		for startEntry < len(entries) {
			dl := displayLines(startEntry, cursor+1)
			if dl <= height {
				break
			}
			startEntry++
		}
		// Apply scrolloff: ensure at least scrollOff entries visible after cursor.
		// Scroll down (increase startEntry) if needed to show more below cursor.
		if cursor+scrollOff < len(entries) {
			for startEntry < len(entries) {
				dl := displayLines(startEntry, cursor+scrollOff+1)
				if dl <= height {
					break
				}
				startEntry++
			}
		}
		// Apply scrolloff: ensure at least scrollOff entries visible before cursor.
		if cursor-scrollOff >= 0 && startEntry > cursor-scrollOff {
			startEntry = cursor - scrollOff
			if startEntry < 0 {
				startEntry = 0
			}
		}
		// Clamp startEntry so we never show empty trailing space.
		// Walk backwards from the end to find the maximum valid startEntry.
		for startEntry > 0 && displayLines(startEntry, len(entries)) < height {
			startEntry--
		}
	}

	// Find end entry that fits within height.
	endEntry := startEntry
	usedLines := 0
	for endEntry < len(entries) {
		entryLines := 0
		e := entries[endEntry]
		if e.hasSep {
			entryLines++
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

	name := displayName

	// Prepend icon if present (resolved based on IconMode).
	icon := resolveIcon(item.Icon)
	if icon != "" {
		name = IconStyle.Render(icon) + " " + name
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

// RenderYAMLContent renders arbitrary YAML content with syntax highlighting, truncated to fit.
func RenderYAMLContent(content string, width, height int) string {
	lines := strings.Split(content, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	var b strings.Builder
	for i, line := range lines {
		b.WriteString(HighlightYAMLLine(Truncate(line, width)))
		if i < len(lines)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// RenderContainerDetail renders detailed information about a container.
func RenderContainerDetail(item *model.Item, width, height int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))

	// Collect all rows as (key, value, style) tuples.
	type row struct {
		key   string
		value string
		style lipgloss.Style // value style override
	}
	rows := make([]row, 0, 10)

	rows = append(rows, row{"Name", item.Name, valueStyle})
	// Show container type if not a regular container.
	switch item.Category {
	case "Init Containers":
		rows = append(rows, row{"Type", "Init Container", DimStyle})
	case "Sidecar Containers":
		rows = append(rows, row{"Type", "Sidecar Container", DimStyle})
	}
	rows = append(rows, row{"Status", item.Status, StatusStyle(item.Status)})
	if item.Extra != "" {
		rows = append(rows, row{"Image", item.Extra, DimStyle})
	}
	if item.Ready != "" {
		rows = append(rows, row{"Ready", item.Ready, valueStyle})
	}
	if item.Restarts != "" {
		rows = append(rows, row{"Restarts", item.Restarts, valueStyle})
	}
	if item.Age != "" {
		rows = append(rows, row{"Age", item.Age, AgeStyle(item.Age)})
	}

	// Additional columns (reason, message, resources, ports, etc.).
	for _, kv := range item.Columns {
		if strings.HasPrefix(kv.Key, "__") || strings.HasPrefix(kv.Key, "owner:") || strings.HasPrefix(kv.Key, "secret:") || strings.HasPrefix(kv.Key, "data:") {
			continue
		}
		rows = append(rows, row{kv.Key, kv.Value, valueStyle})
	}

	// Find max key length for alignment.
	maxKeyLen := 0
	for _, r := range rows {
		if len(r.key) > maxKeyLen {
			maxKeyLen = len(r.key)
		}
	}

	// Render all rows with aligned labels.
	lines := make([]string, 0, len(rows)+3)
	lines = append(lines, DimStyle.Bold(true).Render("CONTAINER DETAILS"))
	lines = append(lines, "")
	for _, r := range rows {
		if len(lines) >= height-1 {
			break
		}
		padded := r.key + ": " + strings.Repeat(" ", maxKeyLen-len(r.key))
		lines = append(lines, labelStyle.Render(padded)+r.style.Render(r.value))
	}

	return strings.Join(lines, "\n")
}

// HighlightYAMLLine applies syntax highlighting to a single YAML line.
func HighlightYAMLLine(line string) string {
	trimmed := strings.TrimLeft(line, " ")
	indent := line[:len(line)-len(trimmed)]

	// Comment lines.
	if strings.HasPrefix(trimmed, "#") {
		return YamlCommentStyle.Render(line)
	}

	// Lines with key: value.
	if colonIdx := strings.Index(trimmed, ":"); colonIdx > 0 {
		key := trimmed[:colonIdx]
		rest := trimmed[colonIdx:]

		// Check this looks like a YAML key (no spaces in key part before colon,
		// or it's a simple key with dash prefix).
		keyPart := strings.TrimPrefix(key, "- ")
		if !strings.Contains(keyPart, " ") || strings.HasPrefix(key, "- ") {
			styledKey := YamlKeyStyle.Render(key)
			if len(rest) > 1 {
				colon := YamlPunctuationStyle.Render(":")
				value := YamlValueStyle.Render(rest[1:])
				return indent + styledKey + colon + value
			}
			return indent + styledKey + YamlPunctuationStyle.Render(":")
		}
	}

	// List item marker.
	if strings.HasPrefix(trimmed, "- ") {
		marker := YamlPunctuationStyle.Render("- ")
		return indent + marker + YamlValueStyle.Render(trimmed[2:])
	}

	return YamlValueStyle.Render(line)
}

// HighlightSearchInLine highlights search matches in a line.
// If isCurrent is true, uses a more prominent style for the current match.
func HighlightSearchInLine(line, query string, isCurrent bool) string {
	if query == "" {
		return HighlightYAMLLine(line)
	}
	lower := strings.ToLower(line)
	queryLower := strings.ToLower(query)
	idx := strings.Index(lower, queryLower)
	if idx < 0 {
		return HighlightYAMLLine(line)
	}
	style := SearchHighlightStyle
	if isCurrent {
		style = SelectedSearchHighlightStyle
	}
	var result strings.Builder
	for idx >= 0 {
		// Render the non-matching part with YAML highlighting.
		result.WriteString(YamlValueStyle.Render(line[:idx]))
		result.WriteString(style.Render(line[idx : idx+len(query)]))
		line = line[idx+len(query):]
		lower = lower[idx+len(queryLower):]
		idx = strings.Index(lower, queryLower)
	}
	result.WriteString(YamlValueStyle.Render(line))
	return result.String()
}

// FormatItemNameOnly formats an item showing only its name and icon (no status, age, etc.).
// Used for parent and child columns where details are not needed.
func FormatItemNameOnly(item model.Item, width int) string {
	displayName := item.Name
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	// Build deprecation suffix (styled).
	deprecationSuffix := ""
	deprecationW := 0
	if item.Deprecated {
		deprecationSuffix = DeprecationStyle.Render(" ⚠")
		deprecationW = lipgloss.Width(deprecationSuffix)
	}

	resolvedIcon := resolveIcon(item.Icon)

	if item.Status == "current" {
		prefix := CurrentMarkerStyle.Render("* ")
		prefixW := lipgloss.Width(prefix)
		if resolvedIcon != "" {
			icon := IconStyle.Render(resolvedIcon) + " "
			iconW := lipgloss.Width(icon)
			remaining := width - prefixW - iconW - deprecationW
			if remaining < 1 {
				remaining = 1
			}
			return prefix + icon + Truncate(displayName, remaining) + deprecationSuffix
		}
		remaining := width - prefixW - deprecationW
		if remaining < 1 {
			remaining = 1
		}
		return prefix + Truncate(displayName, remaining) + deprecationSuffix
	}

	if resolvedIcon != "" {
		icon := IconStyle.Render(resolvedIcon) + " "
		iconW := lipgloss.Width(icon)
		remaining := width - iconW - deprecationW
		if remaining < 1 {
			remaining = 1
		}
		return icon + Truncate(displayName, remaining) + deprecationSuffix
	}

	remaining := width - deprecationW
	if remaining < 1 {
		remaining = 1
	}
	return Truncate(displayName, remaining) + deprecationSuffix
}

// FormatItemNameOnlyPlain formats an item showing only name and icon, without ANSI styling.
// Used for highlighted items in parent/child columns.
func FormatItemNameOnlyPlain(item model.Item, width int) string {
	displayName := item.Name
	if item.Namespace != "" {
		displayName = item.Namespace + "/" + displayName
	}

	// Build deprecation suffix (plain text).
	deprecationSuffix := ""
	deprecationW := 0
	if item.Deprecated {
		deprecationSuffix = " ⚠"
		deprecationW = lipgloss.Width(deprecationSuffix)
	}

	resolvedIcon := resolveIcon(item.Icon)

	if item.Status == "current" {
		prefix := "* "
		prefixW := len(prefix)
		if resolvedIcon != "" {
			icon := resolvedIcon + " "
			iconW := lipgloss.Width(icon)
			remaining := width - prefixW - iconW - deprecationW
			if remaining < 1 {
				remaining = 1
			}
			return prefix + icon + Truncate(displayName, remaining) + deprecationSuffix
		}
		remaining := width - prefixW - deprecationW
		if remaining < 1 {
			remaining = 1
		}
		return prefix + Truncate(displayName, remaining) + deprecationSuffix
	}

	if resolvedIcon != "" {
		icon := resolvedIcon + " "
		iconW := lipgloss.Width(icon)
		remaining := width - iconW - deprecationW
		if remaining < 1 {
			remaining = 1
		}
		return icon + Truncate(displayName, remaining) + deprecationSuffix
	}

	remaining := width - deprecationW
	if remaining < 1 {
		remaining = 1
	}
	return Truncate(displayName, remaining) + deprecationSuffix
}

// RenderTable renders items in a table format with column headers for resource views.
// headerLabel is used as the first column header; defaults to "NAME" if empty.
func RenderTable(headerLabel string, items []model.Item, cursor int, width, height int, loading bool, spinnerView string, errMsg string, showMarker ...bool) string {
	var b strings.Builder

	if len(items) == 0 {
		switch {
		case loading:
			b.WriteString(DimStyle.Render(spinnerView + " Loading..."))
		case errMsg != "":
			b.WriteString(ErrorStyle.Render(Truncate(errMsg, width)))
		default:
			b.WriteString(DimStyle.Render("No resources found"))
		}
		return b.String()
	}

	// Detect which detail columns have data.
	hasNs, hasReady, hasRestarts, hasAge, hasStatus := false, false, false, false, false
	for _, item := range items {
		if item.Namespace != "" {
			hasNs = true
		}
		if item.Ready != "" {
			hasReady = true
		}
		if item.Restarts != "" {
			hasRestarts = true
		}
		if item.Age != "" {
			hasAge = true
		}
		if item.Status != "" {
			hasStatus = true
		}
	}

	// Content-aware column widths: size each column based on actual data,
	// then give all remaining space to the name column so the table fills
	// the full available width.
	nsW, readyW, restartsW, ageW, statusW := 0, 0, 0, 0, 0
	if hasNs {
		nsW = len("NAMESPACE") // minimum: header width
		for _, item := range items {
			if w := len(item.Namespace); w > nsW {
				nsW = w
			}
		}
		nsW++ // spacing
		// Cap to avoid extremely long namespaces dominating the layout.
		if nsW > 30 {
			nsW = 30
		}
	}
	if hasReady {
		readyW = len("READY") // 5
		for _, item := range items {
			if w := len(item.Ready); w > readyW {
				readyW = w
			}
		}
		readyW++ // spacing
	}
	// Check if any item has a recent restart — if so, reserve arrow space for all.
	anyRecentRestart := false
	if hasRestarts {
		restartsW = len("RS") + 1 // 3
		for _, item := range items {
			if rc, _ := strconv.Atoi(item.Restarts); rc > 0 {
				if !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour {
					anyRecentRestart = true
					break
				}
			}
		}
		for _, item := range items {
			w := len(item.Restarts)
			if anyRecentRestart {
				w++ // reserve space for "↑" indicator (or placeholder)
			}
			if w >= restartsW {
				restartsW = w + 1
			}
		}
	}
	if hasAge {
		ageW = len("AGE") + 1 // 4
		for _, item := range items {
			if w := len(item.Age); w >= ageW {
				ageW = w + 1
			}
		}
		if ageW > 10 {
			ageW = 10
		}
	}
	if hasStatus {
		statusW = len("STATUS") // minimum for header
		for _, item := range items {
			if w := len(item.Status); w > statusW {
				statusW = w
			}
		}
		statusW++ // spacing
		if statusW > 20 {
			statusW = 20
		}
	}
	// Reserve space for the selection marker column in the focus pane
	// so the table doesn't shift when selections are made.
	wantMarker := len(showMarker) == 0 || showMarker[0]
	markerColW := 0
	if wantMarker {
		markerColW = 2
	}

	// Collect extra columns from item data (additionalPrinterColumns).
	// Derive the resource kind from the first item (all items in a table share the same kind).
	tableKind := ""
	if len(items) > 0 {
		tableKind = items[0].Kind
	}
	extraCols := collectExtraColumns(items, width, nsW+readyW+restartsW+ageW+statusW+markerColW, tableKind)

	// Calculate total extra column width.
	extraTotalW := 0
	for _, ec := range extraCols {
		extraTotalW += ec.width
	}

	// Name column gets all remaining space so the table fills the full width.
	nameW := width - nsW - readyW - restartsW - ageW - statusW - markerColW - extraTotalW
	if nameW < 10 {
		nameW = 10
	}

	// Render header.
	if headerLabel == "" {
		headerLabel = "NAME"
	}
	hdrMarker := ""
	if wantMarker {
		hdrMarker = "  "
	}
	hdr := hdrMarker + formatTableRowWithExtra(headerLabel, "NAMESPACE", "READY", "RS", "STATUS", "AGE",
		nameW, nsW, readyW, restartsW, statusW, ageW, hasNs, hasReady, hasRestarts, hasStatus, hasAge,
		extraCols, nil)
	b.WriteString(DimStyle.Bold(true).Render(Truncate(hdr, width)))
	height-- // header takes one line

	// Detect category transitions for category headers.
	hasCategories := false
	categoryForItem := make([]string, len(items)) // category header to show before item i, or ""
	{
		lastCat := ""
		for i, item := range items {
			if item.Category != "" && item.Category != lastCat {
				categoryForItem[i] = item.Category
				if lastCat != "" {
					hasCategories = true // at least 2 different categories
				}
				lastCat = item.Category
			}
		}
		// Only show category headers if there are multiple distinct categories.
		if !hasCategories {
			for i := range categoryForItem {
				categoryForItem[i] = ""
			}
		}
	}

	// Count extra lines consumed by category headers in a range.
	categoryLines := func(start, end int) int {
		n := 0
		for i := start; i < end && i < len(items); i++ {
			if categoryForItem[i] != "" {
				n++ // category header line
				if i > 0 {
					n++ // separator line before category
				}
			}
		}
		return n
	}

	// Total display lines = items + category header lines.
	totalDisplayLines := len(items) + categoryLines(0, len(items))

	// Scrolling: ensure cursor is visible with scrolloff padding.
	scrollOff := 10
	// Disable or reduce scrolloff when all items fit the visible area.
	if totalDisplayLines <= height {
		scrollOff = 0
	} else if maxSO := (height - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}
	startIdx := 0
	if cursor >= 0 {
		// displayLinesUpTo returns how many display lines items [start..idx] occupy.
		displayLinesUpTo := func(start, idx int) int {
			return (idx - start + 1) + categoryLines(start, idx+1)
		}
		// Ensure cursor is visible.
		for startIdx < len(items) && displayLinesUpTo(startIdx, cursor) > height {
			startIdx++
		}
		// Apply scrolloff: show items after cursor.
		if cursor+scrollOff < len(items) {
			for startIdx < len(items) && displayLinesUpTo(startIdx, cursor+scrollOff) > height {
				startIdx++
			}
		}
		// Apply scrolloff: show items before cursor.
		if cursor-scrollOff >= 0 && startIdx > cursor-scrollOff {
			startIdx = cursor - scrollOff
			if startIdx < 0 {
				startIdx = 0
			}
		}
		// Clamp so we don't show empty trailing space.
		for startIdx > 0 && (len(items)-startIdx+categoryLines(startIdx, len(items))) < height {
			startIdx--
		}
	}

	// Determine endIdx that fits within height.
	usedLines := 0
	endIdx := startIdx
	for endIdx < len(items) {
		extraLines := 0
		if categoryForItem[endIdx] != "" {
			extraLines++
			if endIdx > 0 {
				extraLines++
			}
		}
		if usedLines+1+extraLines > height {
			break
		}
		usedLines += 1 + extraLines
		endIdx++
	}

	for i := startIdx; i < endIdx; i++ {
		item := items[i]

		// Render category header if this item starts a new category.
		if hasCategories && categoryForItem[i] != "" {
			if i > startIdx {
				b.WriteString("\n") // separator line
			}
			b.WriteString("\n" + CategoryStyle.Render(Truncate(categoryForItem[i], width)))
		}

		b.WriteString("\n")

		ns := item.Namespace
		if ns == "" && hasNs {
			ns = "-"
		}

		displayName := item.Name
		if icon := resolveIcon(item.Icon); icon != "" {
			displayName = icon + " " + item.Name
		}

		selected := isItemSelected(item)

		if i == cursor {
			// Selection marker (plain text for cursor row).
			markerPrefix := ""
			if wantMarker {
				markerPrefix = "  "
				if selected {
					markerPrefix = selectionMarker
				}
			}
			// Preprocess restarts value to match styled rendering:
			// add "↑" prefix for recent restarts, or " " placeholder when
			// other items have recent restarts (so the column aligns).
			cursorRestarts := item.Restarts
			if hasRestarts {
				restartCount, _ := strconv.Atoi(item.Restarts)
				recentRestart := !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour
				if restartCount > 0 && recentRestart {
					cursorRestarts = "↑" + item.Restarts
				} else if anyRecentRestart {
					cursorRestarts = " " + item.Restarts
				}
			}
			// Selected row: plain text, no inner styles.
			row := markerPrefix + formatTableRowWithExtra(displayName, ns, item.Ready, cursorRestarts, item.Status, item.Age,
				nameW, nsW, readyW, restartsW, statusW, ageW, hasNs, hasReady, hasRestarts, hasStatus, hasAge,
				extraCols, &item)
			// Apply search/filter highlight on the selected row with contrasting style.
			if ActiveHighlightQuery != "" {
				row = highlightNameSelected(row, ActiveHighlightQuery)
			}
			// Pad to full width for clean highlight.
			lineW := lipgloss.Width(row)
			if lineW < width {
				row += strings.Repeat(" ", width-lineW)
			}
			b.WriteString(SelectedStyle.MaxWidth(width).Render(row))
		} else {
			// Selection marker (styled for non-cursor rows).
			markerPrefix := ""
			if wantMarker {
				markerPrefix = "  "
				if selected {
					markerPrefix = SelectionMarkerStyle.Render(selectionMarker)
				}
			}
			// Non-selected row: apply styling.
			b.WriteString(markerPrefix + formatTableRowStyledWithExtra(item, nameW, nsW, readyW, restartsW, statusW, ageW,
				hasNs, hasReady, hasRestarts, hasStatus, hasAge, extraCols, anyRecentRestart))
		}
	}
	return b.String()
}

// formatTableRow builds a plain text table row.
func formatTableRow(name, ns, ready, restarts, status string,
	nameW, nsW, readyW, restartsW, statusW int,
	hasNs, hasReady, hasRestarts, hasStatus bool) string {
	var parts []string
	if hasNs {
		parts = append(parts, padRight(Truncate(ns, nsW), nsW))
	}
	parts = append(parts, padRight(Truncate(name, nameW), nameW))
	if hasReady {
		parts = append(parts, padRight(ready, readyW))
	}
	if hasRestarts {
		parts = append(parts, padRight(restarts, restartsW))
	}
	if hasStatus {
		parts = append(parts, padRight(Truncate(status, statusW), statusW))
	}
	// Age is appended later in formatTableRowWithExtra, after extra columns.
	return strings.Join(parts, "")
}

// formatTableRowStyled builds a styled table row with colored status and icon.
// anyRecentRestart indicates whether any item in the table has a recent restart,
// which controls whether a " " placeholder is added for alignment in the restarts column.
func formatTableRowStyled(item model.Item, nameW, nsW, readyW, restartsW, statusW int,
	hasNs, hasReady, hasRestarts, hasStatus bool, anyRecentRestart bool) string {
	var parts []string

	// Namespace comes first when shown.
	if hasNs {
		ns := item.Namespace
		if ns == "" {
			ns = "-"
		}
		parts = append(parts, DimStyle.Render(padRight(Truncate(ns, nsW), nsW)))
	}

	// Name with optional icon styling.
	// Succeeded/Completed pods get dimmed names.
	isDimmed := item.Status == "Succeeded" || item.Status == "Completed"
	nameStyle := NormalStyle
	if isDimmed {
		nameStyle = DimStyle
	}
	if resolvedIcon := resolveIcon(item.Icon); resolvedIcon != "" {
		iconSt := IconStyle
		if isDimmed {
			iconSt = DimStyle
		}
		icon := iconSt.Render(resolvedIcon) + " "
		iconVisualW := lipgloss.Width(icon)
		nameRemaining := nameW - iconVisualW
		if nameRemaining < 1 {
			nameRemaining = 1
		}
		namePart := Truncate(item.Name, nameRemaining)
		if ActiveHighlightQuery != "" {
			namePart = highlightName(namePart, ActiveHighlightQuery)
		}
		nameVisualW := lipgloss.Width(namePart)
		pad := nameW - iconVisualW - nameVisualW
		if pad < 0 {
			pad = 0
		}
		if isDimmed {
			namePart = DimStyle.Render(namePart)
		}
		parts = append(parts, icon+namePart+strings.Repeat(" ", pad))
	} else {
		displayName := Truncate(item.Name, nameW)
		if ActiveHighlightQuery != "" {
			displayName = highlightName(displayName, ActiveHighlightQuery)
		}
		parts = append(parts, nameStyle.Render(padRight(displayName, nameW)))
	}

	if hasReady {
		parts = append(parts, DimStyle.Render(padRight(item.Ready, readyW)))
	}
	if hasRestarts {
		restartCount, _ := strconv.Atoi(item.Restarts)
		recentRestart := !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour
		switch {
		case restartCount > 0 && recentRestart:
			restartText := "↑" + item.Restarts
			if restartCount >= 5 {
				parts = append(parts, ErrorStyle.Render(padRight(restartText, restartsW)))
			} else {
				parts = append(parts, StatusFailed.Render(padRight(restartText, restartsW)))
			}
		case anyRecentRestart:
			// Use " " prefix as placeholder to align with rows that have "↑".
			parts = append(parts, DimStyle.Render(padRight(" "+item.Restarts, restartsW)))
		default:
			parts = append(parts, DimStyle.Render(padRight(item.Restarts, restartsW)))
		}
	}
	if hasStatus {
		parts = append(parts, StatusStyle(item.Status).Render(padRight(Truncate(item.Status, statusW), statusW)))
	}
	// Age is appended later in formatTableRowStyledWithExtra, after extra columns.

	return strings.Join(parts, "")
}

// extraColumn represents an additional column discovered from item data.
type extraColumn struct {
	key      string // column key (e.g., "IP", "Node")
	width    int    // display width for this column
	hasArrow bool   // true if any value in this column has a trend arrow
}

// collectExtraColumns discovers which extra columns to show based on item data and config.
// usedWidth is the width already consumed by fixed columns (excluding name).
// kind is the resource Kind (e.g. "Pod") used to resolve per-type column overrides.
func collectExtraColumns(items []model.Item, totalWidth, usedWidth int, kind string) []extraColumn {
	// Collect all available column keys and their max value widths.
	type colInfo struct {
		key      string
		maxValW  int
		count    int
		hasArrow bool // true if any value in this column has a trend arrow
	}
	seen := make(map[string]*colInfo)
	var order []string
	for _, item := range items {
		for _, kv := range item.Columns {
			info, ok := seen[kv.Key]
			if !ok {
				info = &colInfo{key: kv.Key}
				seen[kv.Key] = info
				order = append(order, kv.Key)
			}
			info.count++
			if strings.HasPrefix(kv.Value, "↑ ") || strings.HasPrefix(kv.Value, "↓ ") {
				info.hasArrow = true
			}
			valW := lipgloss.Width(kv.Value)
			if valW > info.maxValW {
				info.maxValW = valW
			}
		}
	}

	if len(order) == 0 {
		return nil
	}

	// Determine which columns to include based on config.
	// Per-resource-type columns take precedence over global columns.
	configCols := ColumnsForKind(kind)
	var candidates []string
	switch {
	case len(configCols) > 0:
		if len(configCols) == 1 && configCols[0] == "*" {
			candidates = order
		} else {
			for _, cfgKey := range configCols {
				if _, ok := seen[cfgKey]; ok {
					candidates = append(candidates, cfgKey)
				}
			}
		}
	default:
		// Auto-detect: show columns that have data for at least 20% of items (or at least 1).
		// Exclude columns that are too verbose or not useful in a table view.
		// In fullscreen mode, show more columns (IP, Node, etc.).
		var blocked map[string]bool
		// Raw request/limit columns are data-only (used to compute CPU/R, CPU/L, MEM/R, MEM/L).
		// Always block them from displaying in the table.
		rawMetricsCols := map[string]bool{
			"CPU Req": true, "CPU Lim": true, "Mem Req": true, "Mem Lim": true,
			"CPU Alloc": true, "Mem Alloc": true,
		}
		if ActiveFullscreenMode {
			blocked = map[string]bool{
				"Health Message": true, "Keys": true,
				"Service Account": true, "Images": true, "Image": true,
				// ArgoCD: Health and Sync are redundant with Status field.
				"Health": true, "Sync": true,
				"Path": true,
			}
		} else {
			blocked = map[string]bool{
				"IP": true, "Images": true, "Image": true,
				"Host IP": true, "Pod IP": true, "Cluster IP": true,
				"Repo": true, "Path": true, "Dest Server": true,
				"Health Message": true, "Keys": true,
				"Service Account": true, "Node": true,
				"QoS": true, "Priority Class": true,
				// ArgoCD: hide in normal view, show Dest Server/NS only in fullscreen.
				"Health": true, "Sync": true, "Dest NS": true,
				"Sync Message": true, "Sync Errors": true,
				// Events: Message and Source are too verbose for table view.
				"Message": true, "Source": true,
			}
		}
		for k, v := range rawMetricsCols {
			blocked[k] = v
		}
		threshold := len(items) / 5
		if threshold < 1 {
			threshold = 1
		}
		for _, key := range order {
			if blocked[key] || strings.HasPrefix(key, "__") || strings.HasPrefix(key, "secret:") || strings.HasPrefix(key, "owner:") || strings.HasPrefix(key, "data:") {
				continue
			}
			info := seen[key]
			if info.count >= threshold {
				candidates = append(candidates, key)
			}
		}
	}

	if len(candidates) == 0 {
		return nil
	}

	// Calculate available width for extra columns (leave at least 20 chars for name).
	minNameW := 20
	available := totalWidth - usedWidth - minNameW
	if available < 8 {
		return nil
	}

	result := make([]extraColumn, 0, len(candidates))
	remainingW := available
	maxColW := 20
	if ActiveFullscreenMode {
		maxColW = 40
	}
	for _, key := range candidates {
		info := seen[key]
		// Column width: max of header length and value length, capped, plus 1 for spacing.
		colW := len(key)
		maxVal := info.maxValW
		// When some values have arrows, non-arrow values need a placeholder space.
		// The arrow values already include the arrow in their visual width,
		// so ensure non-arrow values get +1 to match.
		if info.hasArrow {
			maxVal++ // reserve space for placeholder on non-arrow rows
		}
		if maxVal > colW {
			colW = maxVal
		}
		if colW > maxColW {
			colW = 20
		}
		colW++ // spacing
		if colW > remainingW {
			break
		}
		result = append(result, extraColumn{key: key, width: colW, hasArrow: info.hasArrow})
		remainingW -= colW
	}

	return result
}

// getExtraColumnValue retrieves the value for a given column key from an item.
func getExtraColumnValue(item *model.Item, key string) string {
	if item == nil {
		return ""
	}
	for _, kv := range item.Columns {
		if kv.Key == key {
			return kv.Value
		}
	}
	return ""
}

// formatTableRowWithExtra builds a plain text table row including extra columns.
// When item is nil, header values from extraCols keys are used.
func formatTableRowWithExtra(name, ns, ready, restarts, status, age string,
	nameW, nsW, readyW, restartsW, statusW, ageW int,
	hasNs, hasReady, hasRestarts, hasStatus, hasAge bool,
	extraCols []extraColumn, item *model.Item) string {

	row := formatTableRow(name, ns, ready, restarts, status,
		nameW, nsW, readyW, restartsW, statusW, hasNs, hasReady, hasRestarts, hasStatus)

	for _, ec := range extraCols {
		var val string
		if item == nil {
			// Header row: use key as header.
			val = ec.key
		} else {
			val = getExtraColumnValue(item, ec.key)
		}
		// Handle arrow values the same way as the styled path:
		// strip the arrow prefix and render it as a separate character,
		// or use a space placeholder for non-arrow rows in arrow columns.
		switch {
		case strings.HasPrefix(val, "↑ ") || strings.HasPrefix(val, "↓ "):
			arrow := string([]rune(val)[0])
			baseVal := val[len("↑ "):]
			row += arrow + padRight(Truncate(baseVal, ec.width-1), ec.width-1)
		case ec.hasArrow:
			// Placeholder space to align with rows that have arrows.
			row += " " + padRight(Truncate(val, ec.width-1), ec.width-1)
		default:
			row += padRight(Truncate(val, ec.width), ec.width)
		}
	}

	// Age comes last, after extra columns.
	if hasAge {
		row += padRight(age, ageW)
	}

	return row
}

// formatTableRowStyledWithExtra builds a styled table row including extra columns.
func formatTableRowStyledWithExtra(item model.Item, nameW, nsW, readyW, restartsW, statusW, ageW int,
	hasNs, hasReady, hasRestarts, hasStatus, hasAge bool,
	extraCols []extraColumn, anyRecentRestart bool) string {

	base := formatTableRowStyled(item, nameW, nsW, readyW, restartsW, statusW,
		hasNs, hasReady, hasRestarts, hasStatus, anyRecentRestart)

	for _, ec := range extraCols {
		val := getExtraColumnValue(&item, ec.key)
		style := resourceColumnStyle(ec.key, val)

		// Color trend arrows in metric values (arrows before value).
		// Use a space placeholder for rows without arrows to keep values aligned.
		switch {
		case strings.HasPrefix(val, "↑ "):
			baseVal := val[len("↑ "):]
			base += ErrorStyle.Render("↑") + style.Render(padRight(Truncate(baseVal, ec.width-1), ec.width-1))
		case strings.HasPrefix(val, "↓ "):
			baseVal := val[len("↓ "):]
			base += StatusRunning.Render("↓") + style.Render(padRight(Truncate(baseVal, ec.width-1), ec.width-1))
		case ec.hasArrow:
			// Placeholder space to align with rows that have arrows.
			base += " " + style.Render(padRight(Truncate(val, ec.width-1), ec.width-1))
		default:
			base += style.Render(padRight(Truncate(val, ec.width), ec.width))
		}
	}

	// Age comes last, after extra columns — colored by age bracket.
	if hasAge {
		base += AgeStyle(item.Age).Render(padRight(item.Age, ageW))
	}

	return base
}

// resourceColumnStyle returns a style for extra columns, colorizing CPU/Mem columns.
func resourceColumnStyle(key, val string) lipgloss.Style {
	switch key {
	case "CPU", "MEM":
		// Usage value: color based on percentage against limit (or request).
		return DimStyle
	case "CPU/R", "CPU/L", "MEM/R", "MEM/L", "CPU%", "MEM%":
		// Percentage columns: color based on percentage value.
		return pctStyle(val)
	case "CPU Req", "CPU Lim", "Mem Req", "Mem Lim", "CPU Alloc", "Mem Alloc":
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	case "Last Sync", "Health", "Sync":
		return StatusStyle(val)
	default:
		return DimStyle
	}
}

// pctStyle returns a colored style based on a percentage string like "42%" or "n/a".
func pctStyle(val string) lipgloss.Style {
	if val == "n/a" || val == "" {
		return DimStyle
	}
	val = strings.TrimSuffix(val, "%")
	pct, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return DimStyle
	}
	switch {
	case pct >= 90:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)
	case pct >= 75:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange)).Bold(true)
	default:
		return DimStyle
	}
}

// parseResourceValue parses a CPU (millicores) or memory (bytes) string back to int64.
func ParseResourceValue(val string, isCPU bool) int64 {
	val = strings.TrimSpace(val)
	if val == "" {
		return 0
	}
	if isCPU {
		// CPU: "100m" or "1.5" (cores)
		if strings.HasSuffix(val, "m") {
			n, _ := strconv.ParseFloat(strings.TrimSuffix(val, "m"), 64)
			return int64(n)
		}
		n, _ := strconv.ParseFloat(val, 64)
		return int64(n * 1000)
	}
	// Memory: "128Mi", "1.5Gi", "1024Ki", "1024B"
	switch {
	case strings.HasSuffix(val, "Gi"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(val, "Gi"), 64)
		return int64(n * 1024 * 1024 * 1024)
	case strings.HasSuffix(val, "Mi"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(val, "Mi"), 64)
		return int64(n * 1024 * 1024)
	case strings.HasSuffix(val, "Ki"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(val, "Ki"), 64)
		return int64(n * 1024)
	case strings.HasSuffix(val, "B"):
		n, _ := strconv.ParseFloat(strings.TrimSuffix(val, "B"), 64)
		return int64(n)
	default:
		n, _ := strconv.ParseFloat(val, 64)
		return int64(n)
	}
}

// padRight pads a string with spaces to reach the target visual width.
func padRight(s string, w int) string {
	vis := lipgloss.Width(s)
	if vis >= w {
		return s
	}
	return s + strings.Repeat(" ", w-vis)
}

// Truncate truncates a string to maxW runes, appending "~" if truncated.
func Truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "~"
	}
	return string(runes[:maxW-1]) + "~"
}

// RenderTabBar renders the tab bar showing tab labels with the active tab highlighted.
func RenderTabBar(tabLabels []string, activeTab, width int) string {
	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSelectedFg)).
		Background(lipgloss.Color(ColorPrimary)).
		Bold(true).
		Padding(0, 1)
	inactiveStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDimmed)).
		Padding(0, 1)
	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBorder))
	sep := separatorStyle.Render(" │ ")
	sepW := lipgloss.Width(sep)

	maxBarW := width - 2

	// Truncate long labels.
	maxLabelLen := maxBarW / max(1, len(tabLabels))
	if maxLabelLen < 8 {
		maxLabelLen = 8
	}

	type renderedTab struct {
		text  string
		width int
	}
	tabs := make([]renderedTab, len(tabLabels))
	for i, label := range tabLabels {
		if len(label) > maxLabelLen {
			label = "…" + label[len(label)-maxLabelLen+1:]
		}
		display := fmt.Sprintf("%d %s", i+1, label)
		var text string
		if i == activeTab {
			text = activeStyle.Render(display)
		} else {
			text = inactiveStyle.Render(display)
		}
		tabs[i] = renderedTab{text: text, width: lipgloss.Width(text)}
	}

	// Check if all tabs fit.
	totalW := 0
	for i, t := range tabs {
		totalW += t.width
		if i < len(tabs)-1 {
			totalW += sepW
		}
	}

	if totalW <= maxBarW {
		var parts []string
		for i, t := range tabs {
			parts = append(parts, t.text)
			if i < len(tabs)-1 {
				parts = append(parts, sep)
			}
		}
		return " " + strings.Join(parts, "")
	}

	// Window around active tab.
	left := activeTab
	right := activeTab
	usedW := tabs[activeTab].width

	for {
		expanded := false
		if left > 0 {
			needed := sepW + tabs[left-1].width
			if usedW+needed <= maxBarW {
				left--
				usedW += needed
				expanded = true
			}
		}
		if right < len(tabs)-1 {
			needed := sepW + tabs[right+1].width
			if usedW+needed <= maxBarW {
				right++
				usedW += needed
				expanded = true
			}
		}
		if !expanded {
			break
		}
	}

	var parts []string
	if left > 0 {
		parts = append(parts, inactiveStyle.Render("◂"))
		parts = append(parts, sep)
	}
	for i := left; i <= right; i++ {
		parts = append(parts, tabs[i].text)
		if i < right {
			parts = append(parts, sep)
		}
	}
	if right < len(tabs)-1 {
		parts = append(parts, sep)
		parts = append(parts, inactiveStyle.Render("▸"))
	}

	return " " + strings.Join(parts, "")
}

// RenderResourceSummary renders a key-value summary of resource fields,
// followed by truncated YAML if space permits.
func RenderResourceSummary(item *model.Item, yaml string, width, height int) string {
	if item == nil || len(item.Columns) == 0 {
		// No summary data, fall back to YAML.
		if yaml != "" {
			return RenderYAMLContent(yaml, width, height)
		}
		return DimStyle.Render("No preview")
	}

	var lines []string

	// Header with resource name/kind.
	nameLabel := item.Kind + ": " + item.Name
	if len(nameLabel) > width-2 {
		nameLabel = nameLabel[:width-5] + "..."
	}
	lines = append(lines, DimStyle.Bold(true).Render(nameLabel))

	// Basic fields: Status, Age, Ready, Restarts (if present).
	if item.Status != "" {
		lines = append(lines, renderKV("Status", item.Status, width))
	}
	if item.Age != "" {
		lines = append(lines, HeaderStyle.Render("Age:")+" "+AgeStyle(item.Age).Render(item.Age))
	}
	if item.Ready != "" {
		lines = append(lines, renderKV("Ready", item.Ready, width))
	}
	if item.Restarts != "" {
		lines = append(lines, renderKV("Restarts", item.Restarts, width))
	}
	if item.Namespace != "" {
		lines = append(lines, renderKV("Namespace", item.Namespace, width))
	}

	// Additional columns (skip secret values unless toggle is on, skip metrics columns
	// as they are shown separately in the resource usage bars).
	// Data/secret fields are collected separately and rendered after a separator.
	metricsKeys := map[string]bool{
		"CPU": true, "CPU/R": true, "CPU/L": true,
		"MEM": true, "MEM/R": true, "MEM/L": true,
		"CPU%": true, "MEM%": true,
		"CPU Req": true, "CPU Lim": true, "Mem Req": true, "Mem Lim": true,
		"CPU Alloc": true, "Mem Alloc": true,
	}
	var dataLines []string
	for _, kv := range item.Columns {
		if metricsKeys[kv.Key] {
			continue
		}
		if strings.HasPrefix(kv.Key, "secret:") {
			label := kv.Key[len("secret:"):]
			if ActiveShowSecretValues {
				dataLines = append(dataLines, renderDataKV(label, kv.Value, width)...)
			} else {
				dataLines = append(dataLines, renderDataKV(label, "********", width)...)
			}
			continue
		}
		if strings.HasPrefix(kv.Key, "data:") {
			label := kv.Key[len("data:"):]
			dataLines = append(dataLines, renderDataKV(label, kv.Value, width)...)
			continue
		}
		if len(lines) < height-2 {
			lines = append(lines, renderKV(kv.Key, kv.Value, width))
		}
	}

	// Render data/secret fields in a separate section with a header.
	if len(dataLines) > 0 && len(lines) < height-2 {
		separator := fmt.Sprintf("── DATA (%d) ", len(dataLines))
		if remaining := min(width-2, 40) - lipgloss.Width(separator); remaining > 0 {
			separator += strings.Repeat("─", remaining)
		}
		lines = append(lines, DimStyle.Render(separator))
		for _, dl := range dataLines {
			if len(lines) >= height-2 {
				break
			}
			lines = append(lines, dl)
		}
	}

	// If we have space left and YAML available, show separator + YAML.
	usedLines := len(lines)
	remaining := height - usedLines - 1 // -1 for separator
	if remaining > 3 && yaml != "" {
		lines = append(lines, DimStyle.Render(strings.Repeat("─", min(width-2, 40))))
		yamlContent := RenderYAMLContent(yaml, width, remaining-1)
		lines = append(lines, yamlContent)
	}

	return strings.Join(lines, "\n")
}

// RenderResourceUsage renders CPU and memory usage bars for the preview.
// If request/limit values are zero, usage is shown without a percentage bar.
func RenderResourceUsage(cpuUsed, cpuReq, cpuLim, memUsed, memReq, memLim int64, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true)

	lines := make([]string, 0, 4)
	lines = append(lines, DimStyle.Bold(true).Render("RESOURCE USAGE"))
	lines = append(lines, "")

	// CPU bar.
	cpuLabel := labelStyle.Render("CPU: ")
	cpuBar := renderUsageBar(cpuUsed, cpuReq, cpuLim, width-6, FormatCPU)
	lines = append(lines, cpuLabel+cpuBar)

	// Memory bar.
	memLabel := labelStyle.Render("Mem: ")
	memBar := renderUsageBar(memUsed, memReq, memLim, width-6, FormatMemory)
	lines = append(lines, memLabel+memBar)

	return strings.Join(lines, "\n")
}

// renderUsageBar renders a single usage bar line.
func renderUsageBar(used, req, lim int64, barWidth int, formatFn func(int64) string) string {
	usedStr := formatFn(used)

	// Determine the reference value for percentage (prefer limit, fallback to request).
	ref := lim
	if ref == 0 {
		ref = req
	}

	if ref == 0 {
		// No reference: just show the used value without a bar.
		return usedStr
	}

	pct := float64(used) / float64(ref) * 100
	if pct > 100 {
		pct = 100
	}

	refStr := formatFn(ref)
	suffix := fmt.Sprintf(" %s / %s (%.0f%%)", usedStr, refStr, pct)
	suffixW := lipgloss.Width(suffix)

	// Calculate bar width.
	bw := barWidth - suffixW - 2 // 2 for brackets
	if bw < 5 {
		bw = 5
	}

	filled := int(float64(bw) * pct / 100)
	if filled > bw {
		filled = bw
	}
	empty := bw - filled

	// Color based on percentage.
	var barColor string
	switch {
	case pct >= 90:
		barColor = ColorError
	case pct >= 75:
		barColor = ColorOrange
	default:
		barColor = ColorDimmed
	}

	barStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(barColor))
	emptyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))

	bar := "[" + barStyle.Render(strings.Repeat("\u2588", filled)) + emptyStyle.Render(strings.Repeat("\u2591", empty)) + "]"
	return bar + suffix
}

// FormatCPU formats millicores into a human-readable string.
func FormatCPU(millis int64) string {
	if millis >= 1000 {
		return fmt.Sprintf("%.1f", float64(millis)/1000)
	}
	return fmt.Sprintf("%dm", millis)
}

// FormatMemory formats bytes into a human-readable string (Ki/Mi/Gi).
func FormatMemory(bytes int64) string {
	switch {
	case bytes >= 1024*1024*1024:
		return fmt.Sprintf("%.1fGi", float64(bytes)/(1024*1024*1024))
	case bytes >= 1024*1024:
		return fmt.Sprintf("%.0fMi", float64(bytes)/(1024*1024))
	case bytes >= 1024:
		return fmt.Sprintf("%.0fKi", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// ComputePctStr computes usage as a percentage of a reference value string.
// Returns "n/a" if reference is empty or zero, otherwise returns e.g. "42%".
func ComputePctStr(used int64, refStr string, isCPU bool) string {
	if refStr == "" {
		return "n/a"
	}
	ref := ParseResourceValue(refStr, isCPU)
	if ref == 0 {
		return "n/a"
	}
	pct := float64(used) / float64(ref) * 100
	return fmt.Sprintf("%.0f%%", pct)
}

// renderKV renders a single key-value pair for the resource summary.
func renderKV(key, value string, width int) string {
	keyStr := HeaderStyle.Render(key + ":")
	keyW := lipgloss.Width(keyStr)
	maxVal := width - keyW - 2
	if maxVal < 4 {
		maxVal = 4
	}
	if len(value) > maxVal {
		value = value[:maxVal-3] + "..."
	}
	return keyStr + " " + DimStyle.Render(value)
}

// renderDataKV renders a key-value pair for ConfigMap/Secret data fields.
// If the value contains newlines, the key is rendered on its own line and
// each subsequent line of the value is indented with two spaces.
// Returns a slice of lines to allow the caller to account for height limits.
func renderDataKV(key, value string, width int) []string {
	// Normalize escaped newline sequences to actual newlines.
	value = strings.ReplaceAll(value, `\n`, "\n")
	value = strings.ReplaceAll(value, `\t`, "\t")

	if !strings.Contains(value, "\n") {
		return []string{renderKV(key, value, width)}
	}
	// Multiline: render key on its own line, then indented value lines.
	header := HeaderStyle.Render(key + ":")
	lines := []string{header}
	indent := "  "
	maxVal := width - len(indent) - 1
	if maxVal < 4 {
		maxVal = 4
	}
	for _, vline := range strings.Split(value, "\n") {
		rendered := vline
		if len(rendered) > maxVal {
			rendered = rendered[:maxVal-3] + "..."
		}
		lines = append(lines, DimStyle.Render(indent+rendered))
	}
	return lines
}

// RenderResourceTree renders a resource relationship tree as an ASCII tree diagram.
func RenderResourceTree(root *model.ResourceNode, width, height int) string {
	if root == nil {
		return DimStyle.Render("No resource tree available")
	}

	var b strings.Builder

	// Title.
	title := SelectedStyle.Bold(true).Render(" Resource Map ")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Render root node. Pass root's own namespace to suppress the namespace annotation
	// on the root itself (the user already knows which resource they selected).
	rootLabel := renderTreeNodeLabel(root, root.Namespace)
	b.WriteString(Truncate(rootLabel, width))
	b.WriteString("\n")

	// Empty line between root and children for better visual separation.
	if len(root.Children) > 0 {
		b.WriteString(DimStyle.Render("│"))
		b.WriteString("\n")
	}

	// Render children recursively.
	lines := renderTreeChildren(root.Children, "", root.Namespace, width)
	for _, line := range lines {
		b.WriteString(line)
		b.WriteString("\n")
	}

	if len(root.Children) == 0 {
		b.WriteString(DimStyle.Render("  (no owned resources)"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderTreeNodeLabel builds the display label for a single tree node.
// parentNamespace is used to conditionally show the namespace when it differs
// from the parent node.
func renderTreeNodeLabel(node *model.ResourceNode, parentNamespace string) string {
	label := OverlayTitleStyle.Render(node.Kind+"/") + NormalStyle.Render(node.Name)

	// Show namespace when it differs from the parent.
	if node.Namespace != "" && node.Namespace != parentNamespace {
		label += " " + DimStyle.Render("(ns: "+node.Namespace+")")
	}

	// Show child count for nodes that have children.
	if len(node.Children) > 0 {
		childKind := node.Children[0].Kind
		label += " " + DimStyle.Render(fmt.Sprintf("(%d %s)", len(node.Children), childKind))
	}

	if node.Status != "" {
		label += " " + StatusStyle(node.Status).Render("["+node.Status+"]")
	}
	return label
}

func renderTreeChildren(children []*model.ResourceNode, prefix, parentNamespace string, width int) []string {
	lines := make([]string, 0, len(children))
	for i, child := range children {
		isLast := i == len(children)-1

		connector := "├── "
		childPrefix := "│   "
		if isLast {
			connector = "└── "
			childPrefix = "    "
		}

		line := DimStyle.Render(prefix+connector) + renderTreeNodeLabel(child, parentNamespace)
		lines = append(lines, Truncate(line, width))

		// Recurse into children.
		if len(child.Children) > 0 {
			subLines := renderTreeChildren(child.Children, prefix+childPrefix, child.Namespace, width)
			lines = append(lines, subLines...)
		}
	}
	return lines
}
