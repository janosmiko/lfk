package ui

import (
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

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
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(BaseBg)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile)).Background(BaseBg)

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
			icon := IconStyle.Render(resolvedIcon + " ")
			iconW := lipgloss.Width(icon)
			remaining := width - prefixW - iconW - deprecationW
			if remaining < 1 {
				remaining = 1
			}
			return prefix + icon + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
		}
		remaining := width - prefixW - deprecationW
		if remaining < 1 {
			remaining = 1
		}
		return prefix + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
	}

	if resolvedIcon != "" {
		icon := IconStyle.Render(resolvedIcon + " ")
		iconW := lipgloss.Width(icon)
		remaining := width - iconW - deprecationW
		if remaining < 1 {
			remaining = 1
		}
		return icon + NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
	}

	remaining := width - deprecationW
	if remaining < 1 {
		remaining = 1
	}
	return NormalStyle.Render(Truncate(displayName, remaining)) + deprecationSuffix
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

// wrapExtraValue splits a value into continuation-line chunks of the given width.
// It returns only the continuation lines (chunks after the first), since the first
// chunk is already rendered by the normal row. Returns nil if no wrapping is needed.
func wrapExtraValue(val string, width int) []string {
	if width <= 0 {
		return nil
	}
	runes := []rune(val)
	if len(runes) <= width {
		return nil
	}
	var lines []string
	for i := width; i < len(runes); i += width {
		end := i + width
		if end > len(runes) {
			end = len(runes)
		}
		lines = append(lines, string(runes[i:end]))
	}
	return lines
}

// itemExtraLines returns how many continuation lines an item needs for wrapping the last extra column.
func itemExtraLines(item *model.Item, extraCols []extraColumn) int {
	if len(extraCols) == 0 || item == nil {
		return 0
	}
	lastCol := extraCols[len(extraCols)-1]
	val := getExtraColumnValue(item, lastCol.key)
	wrapWidth := lastCol.width - 1 // account for spacing
	if wrapWidth <= 0 {
		return 0
	}
	runes := []rune(val)
	if len(runes) <= wrapWidth {
		return 0
	}
	// Number of continuation lines (total lines minus the first one).
	totalLines := (len(runes) + wrapWidth - 1) / wrapWidth
	return totalLines - 1
}

// RenderTable renders items in a table format with column headers for resource views.
// headerLabel is used as the first column header; defaults to "NAME" if empty.
func RenderTable(headerLabel string, items []model.Item, cursor int, width, height int, loading bool, spinnerView string, errMsg string, showMarker ...bool) string {
	var b strings.Builder

	if len(items) == 0 {
		switch {
		case loading:
			b.WriteString(DimStyle.Render(spinnerView+" ") + DimStyle.Render("Loading..."))
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
				if i > start {
					n++ // separator line before category (not for first item in range)
				}
			}
		}
		return n
	}

	// Display lines for a range of items (each item = 1 line + category headers + wrap lines).
	tableDisplayLines := func(from, to int) int {
		base := (to - from) + categoryLines(from, to)
		for i := from; i < to && i < len(items); i++ {
			base += itemExtraLines(&items[i], extraCols)
		}
		return base
	}

	// Scrolling: use vim-style scrolloff for stable viewport.
	scrollOff := 10
	startIdx := 0
	if ActiveMiddleScroll >= 0 {
		startIdx = VimScrollOff(ActiveMiddleScroll, cursor, len(items), height, scrollOff, tableDisplayLines)
		ActiveMiddleScroll = startIdx
	} else {
		// Fallback: calculate from scratch (old behavior).
		totalDisplayLines := tableDisplayLines(0, len(items))
		if totalDisplayLines <= height {
			scrollOff = 0
		} else if maxSO := (height - 1) / 2; scrollOff > maxSO {
			scrollOff = maxSO
		}
		if cursor >= 0 {
			displayLinesUpTo := func(start, idx int) int {
				return tableDisplayLines(start, idx+1)
			}
			for startIdx < len(items) && displayLinesUpTo(startIdx, cursor) > height {
				startIdx++
			}
			if cursor+scrollOff < len(items) {
				for startIdx < len(items) && displayLinesUpTo(startIdx, cursor+scrollOff) > height {
					startIdx++
				}
			}
			if cursor-scrollOff >= 0 && startIdx > cursor-scrollOff {
				startIdx = cursor - scrollOff
				if startIdx < 0 {
					startIdx = 0
				}
			}
			for startIdx > 0 && tableDisplayLines(startIdx, len(items)) < height {
				startIdx--
			}
		}
	}

	// Determine endIdx that fits within height.
	usedLines := 0
	endIdx := startIdx
	for endIdx < len(items) {
		extraLines := 0
		if categoryForItem[endIdx] != "" {
			extraLines++ // category header line
			if endIdx > startIdx {
				extraLines++ // separator line (not shown for first visible item)
			}
		}
		wrapLines := itemExtraLines(&items[endIdx], extraCols)
		if usedLines+1+extraLines+wrapLines > height {
			break
		}
		usedLines += 1 + extraLines + wrapLines
		endIdx++
	}

	// Build display-line-to-item map for mouse click handling.
	// Only build when rendering the active middle column (ActiveMiddleScroll >= 0).
	if ActiveMiddleScroll >= 0 {
		ActiveMiddleLineMap = ActiveMiddleLineMap[:0]
		for i := startIdx; i < endIdx; i++ {
			if hasCategories && categoryForItem[i] != "" {
				if i > startIdx {
					ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1) // separator
				}
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, -1) // category header
			}
			ActiveMiddleLineMap = append(ActiveMiddleLineMap, i) // item line
			extra := itemExtraLines(&items[i], extraCols)
			for range extra {
				ActiveMiddleLineMap = append(ActiveMiddleLineMap, i) // wrap continuation
			}
		}
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

		// Emit continuation lines for wrapped last extra column.
		if len(extraCols) > 0 {
			lastCol := extraCols[len(extraCols)-1]
			val := getExtraColumnValue(&item, lastCol.key)
			wrapWidth := lastCol.width - 1
			if wrapWidth > 0 {
				contLines := wrapExtraValue(val, wrapWidth)
				if len(contLines) > 0 {
					// Calculate padding offset to align with the last extra column.
					padOffset := nsW + nameW + readyW + restartsW + statusW
					if wantMarker {
						padOffset += markerColW
					}
					for j := range len(extraCols) - 1 {
						padOffset += extraCols[j].width
					}
					padding := strings.Repeat(" ", padOffset)

					for _, chunk := range contLines {
						contLine := padding + chunk
						b.WriteString("\n")
						if i == cursor {
							lineW := lipgloss.Width(contLine)
							if lineW < width {
								contLine += strings.Repeat(" ", width-lineW)
							}
							b.WriteString(SelectedStyle.MaxWidth(width).Render(contLine))
						} else {
							b.WriteString(DimStyle.Render(contLine))
						}
					}
				}
			}
		}
	}
	return b.String()
}
