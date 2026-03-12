package ui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
)

// ErrorLogEntry stores a single application log entry with its timestamp and severity level.
type ErrorLogEntry struct {
	Time    time.Time
	Message string
	Level   string // "ERR", "WRN", "INF", "DBG"
}

// RenderNamespaceOverlay renders the namespace selection overlay content.
func RenderNamespaceOverlay(items []model.Item, filter string, cursor int, currentNs string, allNs bool, selectedNamespaces map[string]bool, filterMode bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Namespace"))
	b.WriteString("\n")

	// Filter input.
	if filterMode {
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "\u2588"))
	} else if filter != "" {
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	} else {
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	if items == nil {
		b.WriteString(OverlayDimStyle.Render("Loading namespaces..."))
		return b.String()
	}
	if len(items) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching namespaces"))
		return b.String()
	}

	maxVisible := min(15, len(items))
	scrollOff := 3
	// Disable or reduce scrolloff when all items fit the visible area.
	if len(items) <= maxVisible {
		scrollOff = 0
	} else if maxSO := (maxVisible - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	// Keep cursor at least scrollOff lines from the bottom.
	if cursor-start >= maxVisible-scrollOff && start+maxVisible < len(items) {
		start = cursor - maxVisible + scrollOff + 1
	}
	// Keep cursor at least scrollOff lines from the top.
	if cursor-start < scrollOff && start > 0 {
		start = cursor - scrollOff
		if start < 0 {
			start = 0
		}
	}
	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
		prefix := "  "
		if item.Status == "all" {
			if allNs && len(selectedNamespaces) == 0 {
				prefix = "\u2713 "
			}
		} else if selectedNamespaces != nil && selectedNamespaces[item.Name] {
			prefix = "\u2713 "
		} else if item.Name == currentNs && !allNs && len(selectedNamespaces) == 0 {
			prefix = "* "
		}
		line := prefix + item.Name
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("space: select  c: clear  enter: apply  esc: close"))

	return b.String()
}

// RenderActionOverlay renders the action menu overlay content.
func RenderActionOverlay(items []model.Item, cursor int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Actions"))
	b.WriteString("\n")

	for i, item := range items {
		keyHint := ""
		if item.Status != "" {
			keyHint = OverlayFilterStyle.Render("["+item.Status+"]") + " "
		}
		label := fmt.Sprintf("  %s%s - %s", keyHint, item.Name, item.Extra)
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(label))
		} else {
			b.WriteString(OverlayNormalStyle.Render(label))
		}
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("j/k: navigate  enter/key: select  esc: close"))

	return b.String()
}

// RenderConfirmOverlay renders the delete confirmation overlay content.
func RenderConfirmOverlay(action string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Confirm Delete"))
	b.WriteString("\n\n")
	b.WriteString(OverlayWarningStyle.Render(fmt.Sprintf("Delete %s?", action)))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("Press "))
	b.WriteString(OverlayFilterStyle.Render("y"))
	b.WriteString(OverlayNormalStyle.Render(" to confirm, "))
	b.WriteString(OverlayFilterStyle.Render("n"))
	b.WriteString(OverlayNormalStyle.Render(" to cancel"))
	return b.String()
}

// RenderScaleOverlay renders the scale deployment overlay content.
func RenderScaleOverlay(input string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Scale Deployment"))
	b.WriteString("\n\n")
	b.WriteString(OverlayNormalStyle.Render("Replicas: "))
	if input == "" {
		b.WriteString(OverlayDimStyle.Render("_"))
	} else {
		b.WriteString(OverlayInputStyle.Render(input))
	}
	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("Enter a number, then press Enter"))
	return b.String()
}

// PortInfo represents a discovered port for the port forward overlay.
type PortInfo struct {
	Port     string
	Name     string
	Protocol string
}

// RenderPortForwardOverlay renders the port forward overlay content.
// availablePorts shows discovered ports from the resource.
// cursor indicates which available port is selected (-1 for manual input mode).
func RenderPortForwardOverlay(input string, availablePorts []PortInfo, cursor int, resourceName string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Port Forward"))
	if resourceName != "" {
		b.WriteString(OverlayDimStyle.Render("  " + resourceName))
	}
	b.WriteString("\n\n")

	if len(availablePorts) > 0 {
		b.WriteString(OverlayNormalStyle.Render("Available ports:"))
		b.WriteString("\n")
		for i, p := range availablePorts {
			label := fmt.Sprintf("  %s", p.Port)
			if p.Name != "" {
				label += " (" + p.Name + ")"
			}
			if p.Protocol != "" && p.Protocol != "TCP" {
				label += " [" + p.Protocol + "]"
			}
			if i == cursor {
				b.WriteString(OverlaySelectedStyle.Render(label))
			} else {
				b.WriteString(OverlayNormalStyle.Render(label))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	if cursor >= 0 && cursor < len(availablePorts) {
		// A port is selected from the list.
		b.WriteString(OverlayNormalStyle.Render("Remote port: "))
		b.WriteString(OverlayInputStyle.Render(availablePorts[cursor].Port))
		b.WriteString("\n")
		b.WriteString(OverlayNormalStyle.Render("Local port:  "))
		if input == "" {
			b.WriteString(OverlayDimStyle.Render("(random)"))
		} else {
			b.WriteString(OverlayInputStyle.Render(input))
		}
	} else {
		b.WriteString(OverlayNormalStyle.Render("Port mapping: "))
		if input == "" {
			b.WriteString(OverlayDimStyle.Render("local:remote"))
		} else {
			b.WriteString(OverlayInputStyle.Render(input))
		}
	}
	b.WriteString("\n\n")

	var hints string
	if len(availablePorts) > 0 {
		if cursor >= 0 && cursor < len(availablePorts) && input == "" {
			hints = "j/k: select port  enter: forward (random local port)  type: set local port"
		} else {
			hints = "j/k: select port  enter: forward  type: set local port"
		}
	} else {
		hints = "Format: localPort:remotePort, then press Enter"
	}
	b.WriteString(OverlayDimStyle.Render(hints))
	return b.String()
}

// RenderPortForwardListOverlay renders the port forward list view for the action menu.
func RenderPortForwardListOverlay(items []model.Item, cursor int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Active Port Forwards"))
	b.WriteString("\n")

	if len(items) == 0 {
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("No active port forwards"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("Use 'x' action menu on a pod/service to start one"))
		return b.String()
	}

	for i, item := range items {
		line := fmt.Sprintf("  %s", item.Name)
		if item.Extra != "" {
			line += "  " + OverlayDimStyle.Render(item.Extra)
		}
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			switch item.Status {
			case "Running":
				b.WriteString(OverlayNormalStyle.Render(line))
			case "Stopped":
				b.WriteString(OverlayDimStyle.Render(line))
			case "Failed":
				b.WriteString(OverlayWarningStyle.Render(line))
			default:
				b.WriteString(OverlayNormalStyle.Render(line))
			}
		}
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("s: stop  D: remove  esc: close"))

	return b.String()
}

// RenderContainerSelectOverlay renders the container selection overlay content.
func RenderContainerSelectOverlay(items []model.Item, cursor int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Container"))
	b.WriteString("\n")

	for i, item := range items {
		line := fmt.Sprintf("  %s", item.Name)
		if item.Category != "" && item.Category != "Containers" {
			line += "  (" + item.Category + ")"
		}
		if item.Status != "" {
			line += "  " + item.Status
		}
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < len(items)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("j/k: navigate  enter: select  esc: close"))

	return b.String()
}

// RenderPodSelectOverlay renders the pod selection overlay content.
func RenderPodSelectOverlay(items []model.Item, cursor int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Pod"))
	b.WriteString("\n")

	maxVisible := min(15, len(items))
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		item := items[i]
		line := fmt.Sprintf("  %s", item.Name)
		if item.Status != "" {
			line += "  " + item.Status
		}
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("j/k: navigate  enter: select  esc: close"))

	return b.String()
}

// Bookmark overlay mode constants matching the app-level enum.
const (
	bookmarkModeNormal = 0
	bookmarkModeFilter = 1
)

// RenderBookmarkOverlay renders the bookmark list overlay content.
// mode: 0 = normal, 1 = filter.
func RenderBookmarkOverlay(allBookmarks []model.Bookmark, filter string, cursor, mode int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Bookmarks"))
	b.WriteString("\n")

	// Show mode-specific input line.
	switch mode {
	case bookmarkModeFilter:
		b.WriteString(OverlayFilterStyle.Render("filter> " + filter))
		b.WriteString(OverlayDimStyle.Render("█"))
	default:
		if filter != "" {
			b.WriteString(OverlayDimStyle.Render("filter: "))
			b.WriteString(OverlayFilterStyle.Render(filter))
		}
	}
	b.WriteString("\n\n")

	if len(allBookmarks) == 0 {
		b.WriteString(OverlayDimStyle.Render("No bookmarks yet"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("Press "))
		b.WriteString(OverlayFilterStyle.Render("B"))
		b.WriteString(OverlayDimStyle.Render(" in explorer to bookmark current location"))
		return b.String()
	}

	// Apply filter.
	var bookmarks []model.Bookmark
	if filter == "" {
		bookmarks = allBookmarks
	} else {
		lowerFilter := strings.ToLower(filter)
		for _, bm := range allBookmarks {
			if strings.Contains(strings.ToLower(bm.Name), lowerFilter) {
				bookmarks = append(bookmarks, bm)
			}
		}
	}

	if len(bookmarks) == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching bookmarks"))
		return b.String()
	}

	maxVisible := min(15, len(bookmarks))
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(bookmarks) {
		end = len(bookmarks)
	}

	for i := start; i < end; i++ {
		bm := bookmarks[i]
		name := bm.Name

		if bm.Namespace != "" {
			name += DimStyle.Render(" [" + bm.Namespace + "]")
		}
		// Prefix with a number shortcut (1-9) for the first 9 bookmarks.
		var prefix string
		if i < 9 {
			prefix = fmt.Sprintf("%d ", i+1)
		} else {
			prefix = "  "
		}
		line := fmt.Sprintf("  %s%s", prefix, name)
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")

	// Footer hints based on mode.
	switch mode {
	case bookmarkModeFilter:
		b.WriteString(OverlayDimStyle.Render("type to filter  "))
		b.WriteString(OverlayDimStyle.Render("enter: apply  "))
		b.WriteString(OverlayDimStyle.Render("esc: clear"))
	default:
		b.WriteString(OverlayDimStyle.Render("1-9: jump  "))
		b.WriteString(OverlayDimStyle.Render("enter: jump  "))
		b.WriteString(OverlayDimStyle.Render("/: filter  "))
		b.WriteString(OverlayDimStyle.Render("d: delete  "))
		b.WriteString(OverlayDimStyle.Render("D: delete all  "))
		b.WriteString(OverlayDimStyle.Render("esc: close"))
	}

	return b.String()
}

// RenderTemplateOverlay renders the template selection overlay content.
func RenderTemplateOverlay(templates []model.ResourceTemplate, cursor int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Create from Template"))
	b.WriteString("\n")

	if len(templates) == 0 {
		b.WriteString(OverlayDimStyle.Render("No templates available"))
		return b.String()
	}

	maxVisible := min(20, len(templates))
	start := 0
	if cursor >= maxVisible {
		start = cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(templates) {
		end = len(templates)
	}

	lastCategory := ""
	for i := start; i < end; i++ {
		tmpl := templates[i]
		if tmpl.Category != lastCategory {
			lastCategory = tmpl.Category
			b.WriteString("\n")
			b.WriteString(OverlayDimStyle.Render("  " + tmpl.Category))
			b.WriteString("\n")
		}
		line := fmt.Sprintf("    %s - %s", tmpl.Name, tmpl.Description)
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("enter: select  esc: close"))

	return b.String()
}

// RenderErrorLogOverlay renders the application log overlay showing timestamped
// log entries with level indicators. The scroll parameter controls which portion is visible.
// When showDebug is false, DBG entries are filtered out.
func RenderErrorLogOverlay(entries []ErrorLogEntry, scroll int, height int, showDebug bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Application Log"))
	b.WriteString("\n")

	// Filter entries based on debug visibility.
	var visible []ErrorLogEntry
	for _, e := range entries {
		if e.Level == "DBG" && !showDebug {
			continue
		}
		visible = append(visible, e)
	}

	if len(visible) == 0 {
		if len(entries) > 0 && !showDebug {
			b.WriteString(OverlayDimStyle.Render("No entries (debug logs hidden, press d to show)"))
		} else {
			b.WriteString(OverlayDimStyle.Render("No log entries"))
		}
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("Press "))
		b.WriteString(OverlayFilterStyle.Render("esc"))
		b.WriteString(OverlayDimStyle.Render(" to close"))
		return b.String()
	}

	// Reserve lines for the title (1), blank line before footer (1), footer (1), and border padding.
	maxVisible := max(height-4, 1)

	// Show entries in reverse chronological order (newest first).
	reversed := make([]ErrorLogEntry, len(visible))
	for i, e := range visible {
		reversed[len(visible)-1-i] = e
	}

	// Clamp scroll.
	maxScroll := max(len(reversed)-maxVisible, 0)
	scroll = max(min(scroll, maxScroll), 0)

	end := min(scroll+maxVisible, len(reversed))

	// Level styles.
	errLevelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Bold(true)
	wrnLevelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00")).Bold(true)
	infLevelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#888888"))
	dbgLevelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#6272a4"))

	for i := scroll; i < end; i++ {
		entry := reversed[i]
		ts := OverlayDimStyle.Render(entry.Time.Format("15:04:05"))
		var lvl string
		switch entry.Level {
		case "ERR":
			lvl = errLevelStyle.Render("ERR")
		case "WRN":
			lvl = wrnLevelStyle.Render("WRN")
		case "DBG":
			lvl = dbgLevelStyle.Render("DBG")
		default:
			lvl = infLevelStyle.Render("INF")
		}
		line := fmt.Sprintf("  %s %s %s", ts, lvl, OverlayNormalStyle.Render(entry.Message))
		b.WriteString(line)
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	scrollInfo := fmt.Sprintf("%d entries", len(visible))
	if len(visible) != len(entries) {
		scrollInfo += fmt.Sprintf(" (%d hidden)", len(entries)-len(visible))
	}
	if maxScroll > 0 {
		scrollInfo += fmt.Sprintf(" | scroll %d/%d", scroll+1, maxScroll+1)
	}
	b.WriteString(OverlayDimStyle.Render(scrollInfo))
	b.WriteString("  ")
	debugHint := "d: show debug"
	if showDebug {
		debugHint = "d: hide debug"
	}
	b.WriteString(OverlayDimStyle.Render("j/k: scroll  " + debugHint + "  esc: close"))

	return b.String()
}

// RenderColorschemeOverlay renders the color scheme selector overlay content.
// entries is a list of SchemeEntry (with headers). cursor indexes only selectable entries.
func RenderColorschemeOverlay(entries []SchemeEntry, filter string, cursor int, filterMode bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Color Scheme"))
	b.WriteString("\n")

	// Filter input.
	if filterMode {
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "█"))
	} else if filter != "" {
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	} else {
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	// Build display list: when filtering, skip headers and filter selectable entries.
	type displayItem struct {
		label      string
		isHeader   bool
		selectIdx  int // index among selectable items (-1 for headers)
	}

	var items []displayItem
	selectIdx := 0
	if filter == "" {
		for _, e := range entries {
			if e.IsHeader {
				items = append(items, displayItem{label: e.Name, isHeader: true, selectIdx: -1})
			} else {
				items = append(items, displayItem{label: e.Name, isHeader: false, selectIdx: selectIdx})
				selectIdx++
			}
		}
	} else {
		lowerFilter := strings.ToLower(filter)
		for _, e := range entries {
			if e.IsHeader {
				continue
			}
			if strings.Contains(e.Name, lowerFilter) {
				items = append(items, displayItem{label: e.Name, isHeader: false, selectIdx: selectIdx})
				selectIdx++
			}
		}
	}

	selectableCount := selectIdx
	if selectableCount == 0 {
		b.WriteString(OverlayDimStyle.Render("No matching schemes"))
		return b.String()
	}

	// Scrolling window based on selectable cursor position.
	maxVisible := 20
	// Find the display index of the cursor item.
	cursorDisplayIdx := 0
	for i, it := range items {
		if !it.isHeader && it.selectIdx == cursor {
			cursorDisplayIdx = i
			break
		}
	}

	start := 0
	if cursorDisplayIdx >= maxVisible {
		start = cursorDisplayIdx - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(items) {
		end = len(items)
	}

	for i := start; i < end; i++ {
		it := items[i]
		if it.isHeader {
			b.WriteString("\n")
			b.WriteString(CategoryStyle.Render("── " + it.label + " ──"))
		} else {
			prefix := "  "
			if it.label == ActiveSchemeName {
				prefix = "* "
			}
			line := prefix + it.label
			if it.selectIdx == cursor {
				b.WriteString(OverlaySelectedStyle.Render(line))
			} else {
				b.WriteString(OverlayNormalStyle.Render(line))
			}
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("enter: apply  esc: cancel"))

	return b.String()
}

// FilterPresetEntry holds the display info for a single filter preset in the overlay.
type FilterPresetEntry struct {
	Name        string
	Description string
	Key         string
}

// RenderFilterPresetOverlay renders the quick filter preset selection overlay content.
// activePresetName is the name of the currently active preset (empty if none).
func RenderFilterPresetOverlay(presets []FilterPresetEntry, cursor int, activePresetName string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Quick Filters"))
	b.WriteString("\n\n")

	if len(presets) == 0 {
		b.WriteString(OverlayDimStyle.Render("No filter presets available"))
		return b.String()
	}

	for i, preset := range presets {
		keyHint := OverlayFilterStyle.Render("[" + preset.Key + "]")
		activeMarker := "  "
		if preset.Name == activePresetName {
			activeMarker = OverlayFilterStyle.Render("\u2713 ")
		}
		line := fmt.Sprintf("  %s%s %s  %s", activeMarker, keyHint, preset.Name, OverlayDimStyle.Render(preset.Description))
		if i == cursor {
			b.WriteString(OverlaySelectedStyle.Render(line))
		} else {
			b.WriteString(OverlayNormalStyle.Render(line))
		}
		if i < len(presets)-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("key: apply  enter: apply  esc: close  .: clear"))

	return b.String()
}

// RBACCheckEntry holds RBAC check data for rendering in the overlay.
type RBACCheckEntry struct {
	Verb    string
	Allowed bool
}

// RenderRBACOverlay renders the RBAC permission check overlay content.
func RenderRBACOverlay(results []RBACCheckEntry, kind string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("RBAC Permissions: %s", kind)))
	b.WriteString("\n\n")

	for _, r := range results {
		indicator := OverlayWarningStyle.Render("\u2717") // ✗
		if r.Allowed {
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("\u2713") // ✓
		}
		verb := OverlayNormalStyle.Render(fmt.Sprintf("  %-10s", r.Verb))
		b.WriteString(verb)
		b.WriteString(indicator)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("Press any key to close"))
	return b.String()
}

// RenderBatchLabelOverlay renders the batch label/annotation editor overlay.
func RenderBatchLabelOverlay(mode int, input string, remove bool) string {
	var b strings.Builder

	kindName := "Labels"
	if mode == 1 {
		kindName = "Annotations"
	}
	action := "Add"
	if remove {
		action = "Remove"
	}

	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("%s %s", action, kindName)))
	b.WriteString("\n\n")

	if remove {
		b.WriteString(OverlayNormalStyle.Render("  Enter key to remove:"))
	} else {
		b.WriteString(OverlayNormalStyle.Render("  Enter key=value:"))
	}
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s%s", OverlayInputStyle.Render(input), OverlayDimStyle.Render("█")))
	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("  Tab: toggle add/remove"))
	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("  Enter: apply  Esc: cancel"))
	return b.String()
}

// PodStartupEntry holds pod startup data for rendering, avoiding k8s package import.
type PodStartupEntry struct {
	PodName   string
	Namespace string
	TotalTime time.Duration
	Phases    []StartupPhaseEntry
}

// StartupPhaseEntry represents a single phase in the pod startup sequence for rendering.
type StartupPhaseEntry struct {
	Name     string
	Duration time.Duration
	Status   string // "completed", "in-progress", "unknown"
}

// RenderPodStartupOverlay renders the pod startup analysis overlay content.
func RenderPodStartupOverlay(entry PodStartupEntry) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Pod Startup Analysis"))
	b.WriteString("\n")

	// Pod info header.
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Pod:       %s", entry.PodName)))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Namespace: %s", entry.Namespace)))
	b.WriteString("\n")
	b.WriteString(OverlayNormalStyle.Render(fmt.Sprintf("  Total:     %s", formatDuration(entry.TotalTime))))
	b.WriteString("\n\n")

	if len(entry.Phases) == 0 {
		b.WriteString(OverlayDimStyle.Render("  No startup phases available"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("Press any key to close"))
		return b.String()
	}

	// Find max duration for bar scaling.
	var maxDur time.Duration
	for _, p := range entry.Phases {
		if p.Duration > maxDur {
			maxDur = p.Duration
		}
	}

	// Phase color styles.
	schedulingStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#7aa2f7")) // blue
	pullStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))       // yellow
	initStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#73daca"))       // cyan
	containerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))  // green
	readinessStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))  // purple
	inProgressStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#ff9e64")) // orange
	unknownStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#565f89"))    // dim

	// Max bar width (characters).
	barWidth := 25

	for _, phase := range entry.Phases {
		// Determine color based on phase name and status.
		style := unknownStyle
		if phase.Status == "in-progress" {
			style = inProgressStyle
		} else {
			switch {
			case strings.HasPrefix(phase.Name, "Scheduling"):
				style = schedulingStyle
			case strings.HasPrefix(phase.Name, "Image Pull"):
				style = pullStyle
			case strings.Contains(phase.Name, "Init"):
				style = initStyle
			case strings.Contains(phase.Name, "Container") || strings.HasPrefix(phase.Name, "  container:"):
				style = containerStyle
			case strings.HasPrefix(phase.Name, "Readiness"):
				style = readinessStyle
			case strings.HasPrefix(phase.Name, "  init:"):
				style = initStyle
			}
		}

		// Build duration bar.
		barLen := 0
		if maxDur > 0 {
			barLen = int(float64(barWidth) * float64(phase.Duration) / float64(maxDur))
		}
		if barLen < 1 && phase.Duration > 0 {
			barLen = 1
		}

		bar := strings.Repeat("\u2593", barLen) // medium shade block
		emptyBar := strings.Repeat("\u2591", barWidth-barLen)

		// Status indicator.
		statusIndicator := ""
		switch phase.Status {
		case "in-progress":
			statusIndicator = " \u25cb" // circle
		case "unknown":
			statusIndicator = " ?"
		}

		// Format the phase line.
		nameWidth := 20
		name := phase.Name
		if len(name) > nameWidth {
			name = name[:nameWidth-1] + "\u2026"
		}

		durStr := formatDuration(phase.Duration)
		line := fmt.Sprintf("  %-*s %s%s %7s%s",
			nameWidth, name,
			style.Render(bar),
			OverlayDimStyle.Render(emptyBar),
			durStr,
			statusIndicator,
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Legend.
	b.WriteString(OverlayDimStyle.Render("  "))
	b.WriteString(schedulingStyle.Render("\u2593"))
	b.WriteString(OverlayDimStyle.Render(" schedule  "))
	b.WriteString(pullStyle.Render("\u2593"))
	b.WriteString(OverlayDimStyle.Render(" pull  "))
	b.WriteString(initStyle.Render("\u2593"))
	b.WriteString(OverlayDimStyle.Render(" init  "))
	b.WriteString(containerStyle.Render("\u2593"))
	b.WriteString(OverlayDimStyle.Render(" start  "))
	b.WriteString(readinessStyle.Render("\u2593"))
	b.WriteString(OverlayDimStyle.Render(" ready"))
	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("  "))
	b.WriteString(inProgressStyle.Render("\u25cb"))
	b.WriteString(OverlayDimStyle.Render(" in-progress"))
	b.WriteString("\n\n")
	b.WriteString(OverlayDimStyle.Render("Press any key to close"))

	return b.String()
}

// formatDuration formats a duration into a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d\u00b5s", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		mins := int(d.Minutes())
		secs := int(d.Seconds()) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

// QuotaEntry holds quota data for rendering in the overlay (avoids importing k8s).
type QuotaEntry struct {
	Name      string
	Namespace string
	Resources []QuotaResourceEntry
}

// QuotaResourceEntry holds usage data for a single resource in the quota overlay.
type QuotaResourceEntry struct {
	Name    string
	Hard    string
	Used    string
	Percent float64
}

// RenderQuotaDashboardOverlay renders the namespace resource quota dashboard.
func RenderQuotaDashboardOverlay(quotas []QuotaEntry, width, height int) string {
	var b strings.Builder

	// Determine namespace for the title.
	ns := "all namespaces"
	if len(quotas) > 0 && quotas[0].Namespace != "" {
		ns = quotas[0].Namespace
	}
	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("Resource Quotas - %s", ns)))
	b.WriteString("\n")

	// Bar width adapts to the overlay width. Reserve space for label, percentage, and values.
	barWidth := width - 40
	if barWidth < 10 {
		barWidth = 10
	}
	if barWidth > 40 {
		barWidth = 40
	}

	// Severity color styles.
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#9ece6a"))
	yellowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#f7768e"))

	for i, q := range quotas {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(OverlayFilterStyle.Render("  "+q.Name))
		b.WriteString("\n")

		for _, res := range q.Resources {
			// Resource name, left-aligned.
			nameLabel := fmt.Sprintf("    %-16s", res.Name)
			b.WriteString(OverlayNormalStyle.Render(nameLabel))

			// Build the usage bar.
			filled := int(res.Percent / 100.0 * float64(barWidth))
			if filled > barWidth {
				filled = barWidth
			}
			if filled < 0 {
				filled = 0
			}
			empty := barWidth - filled

			filledStr := strings.Repeat("\u2588", filled)
			emptyStr := strings.Repeat("\u2591", empty)

			// Color by severity.
			var barStyle lipgloss.Style
			switch {
			case res.Percent > 90:
				barStyle = redStyle
			case res.Percent > 70:
				barStyle = yellowStyle
			default:
				barStyle = greenStyle
			}

			bar := fmt.Sprintf("[%s%s]", barStyle.Render(filledStr), OverlayDimStyle.Render(emptyStr))
			pctLabel := fmt.Sprintf(" %3.0f%%", res.Percent)

			b.WriteString(bar)

			// Color the percentage label with the same severity color.
			b.WriteString(barStyle.Render(pctLabel))

			// Used/Hard values.
			valLabel := fmt.Sprintf("  %s / %s", res.Used, res.Hard)
			b.WriteString(OverlayDimStyle.Render(valLabel))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(OverlayDimStyle.Render("  esc/q: close"))

	return b.String()
}

// EventTimelineEntry holds event data for rendering in the timeline overlay.
type EventTimelineEntry struct {
	Timestamp    time.Time
	Type         string // "Normal" or "Warning"
	Reason       string
	Message      string
	Source       string
	Count        int32
	InvolvedName string
	InvolvedKind string
}

// RenderEventTimelineOverlay renders the event timeline overlay content.
// Events are displayed with relative timestamps, type indicators, and scrolling support.
func RenderEventTimelineOverlay(events []EventTimelineEntry, resourceName string, scroll, width, height int) string {
	var b strings.Builder

	title := fmt.Sprintf("Event Timeline - %s", resourceName)
	b.WriteString(OverlayTitleStyle.Render(title))
	b.WriteString("\n")

	if len(events) == 0 {
		b.WriteString(OverlayDimStyle.Render("No events found"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("Press "))
		b.WriteString(OverlayFilterStyle.Render("esc"))
		b.WriteString(OverlayDimStyle.Render(" to close"))
		return b.String()
	}

	// Reserve lines for header, blank line before footer, footer.
	maxVisible := max(height-4, 1)

	// Clamp scroll.
	maxScroll := max(len(events)-maxVisible, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := min(scroll+maxVisible, len(events))

	// Styles for event type indicators.
	normalDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Render("\u25cf") // green filled circle
	warningDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("\u25cf")    // red filled circle
	reasonStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFile))
	sourceStyle := OverlayDimStyle
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))

	// Calculate available width for message truncation.
	// Account for overlay padding (4 chars), border (4 chars), and the prefix content.
	msgMaxWidth := max(width-40, 20)

	for i := scroll; i < end; i++ {
		event := events[i]

		// Relative timestamp.
		ts := relativeTime(event.Timestamp)
		tsStr := OverlayDimStyle.Render(fmt.Sprintf("%-8s", ts))

		// Type indicator.
		dot := normalDot
		if event.Type == "Warning" {
			dot = warningDot
		}

		// Reason.
		reason := reasonStyle.Render(event.Reason)

		// Source.
		src := ""
		if event.Source != "" {
			src = " " + sourceStyle.Render("["+event.Source+"]")
		}

		// Involved object info (show if different from the main resource).
		involved := ""
		if event.InvolvedName != resourceName {
			involved = " " + OverlayDimStyle.Render(event.InvolvedKind+"/"+event.InvolvedName)
		}

		// Count.
		countStr := ""
		if event.Count > 1 {
			countStr = " " + countStyle.Render(fmt.Sprintf("(x%d)", event.Count))
		}

		// Message (truncated if needed).
		msg := event.Message
		if ansi.StringWidth(msg) > msgMaxWidth {
			msg = ansi.Truncate(msg, msgMaxWidth, "...")
		}
		msgStr := OverlayNormalStyle.Render(msg)

		// First line: timestamp, dot, reason, source, involved, count.
		line := fmt.Sprintf("  %s %s %s%s%s%s", tsStr, dot, reason, src, involved, countStr)
		b.WriteString(line)
		b.WriteString("\n")

		// Second line: indented message.
		b.WriteString(fmt.Sprintf("           %s", msgStr))

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")

	// Scroll info + hints.
	scrollInfo := fmt.Sprintf("%d events", len(events))
	if maxScroll > 0 {
		scrollInfo += fmt.Sprintf(" | scroll %d/%d", scroll+1, maxScroll+1)
	}
	b.WriteString(OverlayDimStyle.Render(scrollInfo))
	b.WriteString("  ")
	b.WriteString(OverlayDimStyle.Render("j/k: scroll  g/G: top/bottom  esc: close"))

	return b.String()
}

// relativeTime returns a human-readable relative time string (e.g., "2m ago", "1h ago", "3d ago").
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", max(int(d.Seconds()), 1))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// AlertEntry holds alert data for rendering in the overlay, decoupled from the k8s package.
type AlertEntry struct {
	Name        string
	State       string // "firing", "pending"
	Severity    string // "critical", "warning", "info"
	Summary     string
	Description string
	Since       time.Time
	GrafanaURL  string
}

// RenderAlertsOverlay renders the Prometheus alerts overlay content.
// scroll controls the visible portion; width and height limit the overlay size.
func RenderAlertsOverlay(alerts []AlertEntry, scroll, width, height int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Monitoring Overview — Active Alerts"))
	b.WriteString("\n")

	if len(alerts) == 0 {
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("  No active alerts found"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("  Prometheus was queried in well-known namespaces"))
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("  (monitoring, prometheus, observability, kube-prometheus-stack)"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("  Press "))
		b.WriteString(OverlayFilterStyle.Render("esc"))
		b.WriteString(OverlayDimStyle.Render(" to close"))
		return b.String()
	}

	// Build lines for all alerts, then apply scroll window.
	var lines []string
	for i, alert := range alerts {
		if i > 0 {
			lines = append(lines, "")
		}

		// Severity icon + state + alert name.
		var severityIcon string
		var severityStyle lipgloss.Style
		switch alert.Severity {
		case "critical":
			severityIcon = "\u25cf" // filled circle
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)
		case "warning":
			severityIcon = "\u25cf"
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Bold(true)
		default:
			severityIcon = "\u25cf"
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary))
		}

		stateLabel := alert.State
		var stateStyle lipgloss.Style
		if alert.State == "firing" {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true)
		} else {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))
		}

		header := fmt.Sprintf("  %s %s  %s",
			severityStyle.Render(severityIcon+" "+alert.Severity),
			stateStyle.Render("["+stateLabel+"]"),
			OverlayNormalStyle.Render(alert.Name),
		)
		lines = append(lines, header)

		// Summary.
		if alert.Summary != "" {
			lines = append(lines, "    "+OverlayDimStyle.Render(alert.Summary))
		}

		// Description (truncated if too long).
		if alert.Description != "" {
			desc := alert.Description
			maxDescLen := width - 6
			if maxDescLen > 0 && len(desc) > maxDescLen {
				desc = desc[:maxDescLen-3] + "..."
			}
			lines = append(lines, "    "+OverlayDimStyle.Render(desc))
		}

		// Since time.
		if !alert.Since.IsZero() {
			ago := formatRelativeTime(alert.Since)
			lines = append(lines, "    "+OverlayDimStyle.Render("since "+ago))
		}

		// Grafana link hint.
		if alert.GrafanaURL != "" {
			lines = append(lines, "    "+OverlayFilterStyle.Render("dashboard: "+alert.GrafanaURL))
		}
	}

	// Apply scroll window.
	// Reserve lines for header (1), blank line (1), footer (1).
	maxVisible := max(height-4, 1)

	maxScroll := max(len(lines)-maxVisible, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := min(scroll+maxVisible, len(lines))
	for i := scroll; i < end; i++ {
		b.WriteString("\n")
		b.WriteString(lines[i])
	}

	// Footer.
	b.WriteString("\n\n")
	info := fmt.Sprintf("%d alert(s)", len(alerts))
	if maxScroll > 0 {
		info += fmt.Sprintf(" | scroll %d/%d", scroll+1, maxScroll+1)
	}
	b.WriteString(OverlayDimStyle.Render(info + "  j/k: scroll  esc: close"))

	return b.String()
}

// formatRelativeTime returns a human-readable relative time string.
func formatRelativeTime(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm ago", h, m)
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// NetworkPolicyEntry holds network policy data for rendering, decoupled from the k8s package.
type NetworkPolicyEntry struct {
	Name         string
	Namespace    string
	PodSelector  map[string]string
	PolicyTypes  []string
	IngressRules []NetpolRuleEntry
	EgressRules  []NetpolRuleEntry
	AffectedPods []string
}

// NetpolRuleEntry holds a single ingress/egress rule for rendering.
type NetpolRuleEntry struct {
	Ports []NetpolPortEntry
	Peers []NetpolPeerEntry
}

// NetpolPortEntry holds port information for a network policy rule.
type NetpolPortEntry struct {
	Protocol string
	Port     string
}

// NetpolPeerEntry holds peer information for a network policy rule.
type NetpolPeerEntry struct {
	Type      string
	Selector  map[string]string
	CIDR      string
	Except    []string
	Namespace string
}

// RenderNetworkPolicyOverlay renders the network policy visualizer overlay content.
// The overlay shows pod selector, policy types, affected pods, and a visual diagram
// of ingress/egress rules using box-drawing characters and arrows.
func RenderNetworkPolicyOverlay(info NetworkPolicyEntry, scroll, width, height int) string {
	// Styles for the diagram.
	greenStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	arrowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true)
	boxBorderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPurple))
	cidrStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange))
	sectionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true).Underline(true)

	var lines []string

	// Title.
	lines = append(lines, OverlayTitleStyle.Render(fmt.Sprintf("Network Policy: %s", info.Name)))
	lines = append(lines, OverlayDimStyle.Render(fmt.Sprintf("  Namespace: %s", info.Namespace)))
	lines = append(lines, "")

	// Pod Selector section.
	lines = append(lines, sectionStyle.Render("Pod Selector"))
	if len(info.PodSelector) == 0 {
		lines = append(lines, OverlayDimStyle.Render("  (all pods in namespace)"))
	} else {
		selectorKeys := make([]string, 0, len(info.PodSelector))
		for k := range info.PodSelector {
			selectorKeys = append(selectorKeys, k)
		}
		sort.Strings(selectorKeys)
		for _, k := range selectorKeys {
			lines = append(lines, fmt.Sprintf("  %s", labelStyle.Render(k+"="+info.PodSelector[k])))
		}
	}
	lines = append(lines, "")

	// Affected Pods count.
	podCount := len(info.AffectedPods)
	podCountStr := fmt.Sprintf("%d pod(s)", podCount)
	if podCount == 0 {
		podCountStr = "0 pods (or unable to list)"
	}
	lines = append(lines, fmt.Sprintf("  %s %s",
		OverlayNormalStyle.Render("Affected Pods:"),
		greenStyle.Render(podCountStr)))
	if podCount > 0 && podCount <= 10 {
		for _, name := range info.AffectedPods {
			lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(name)))
		}
	} else if podCount > 10 {
		for _, name := range info.AffectedPods[:10] {
			lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(name)))
		}
		lines = append(lines, fmt.Sprintf("    %s", OverlayDimStyle.Render(fmt.Sprintf("... and %d more", podCount-10))))
	}
	lines = append(lines, "")

	// Policy Types.
	if len(info.PolicyTypes) > 0 {
		lines = append(lines, fmt.Sprintf("  %s %s",
			OverlayNormalStyle.Render("Policy Types:"),
			greenStyle.Render(strings.Join(info.PolicyTypes, ", "))))
	} else {
		lines = append(lines, fmt.Sprintf("  %s %s",
			OverlayNormalStyle.Render("Policy Types:"),
			OverlayDimStyle.Render("(none specified)")))
	}
	lines = append(lines, "")

	// Build the target box content (reused for all rules).
	targetLabel := "(all pods)"
	if len(info.PodSelector) > 0 {
		targetKeys := make([]string, 0, len(info.PodSelector))
		for k := range info.PodSelector {
			targetKeys = append(targetKeys, k)
		}
		sort.Strings(targetKeys)
		parts := make([]string, 0, len(info.PodSelector))
		for _, k := range targetKeys {
			parts = append(parts, k+"="+info.PodSelector[k])
		}
		targetLabel = strings.Join(parts, "\n")
	}

	// --- INGRESS RULES ---
	hasIngress := false
	for _, pt := range info.PolicyTypes {
		if pt == "Ingress" {
			hasIngress = true
			break
		}
	}
	if hasIngress || len(info.IngressRules) > 0 {
		lines = append(lines, sectionStyle.Render("INGRESS RULES"))
		lines = append(lines, "")

		if len(info.IngressRules) == 0 {
			lines = append(lines, OverlayWarningStyle.Render("  No ingress rules = all ingress denied"))
			lines = append(lines, "")
		}

		for i, rule := range info.IngressRules {
			lines = append(lines, OverlayNormalStyle.Render(fmt.Sprintf("  Rule %d:", i+1)))
			ruleLines := renderNetpolRuleDiagram(rule, targetLabel, true, width,
				boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)
			lines = append(lines, ruleLines...)
			lines = append(lines, "")
		}
	}

	// --- EGRESS RULES ---
	hasEgress := false
	for _, pt := range info.PolicyTypes {
		if pt == "Egress" {
			hasEgress = true
			break
		}
	}
	if hasEgress || len(info.EgressRules) > 0 {
		lines = append(lines, sectionStyle.Render("EGRESS RULES"))
		lines = append(lines, "")

		if len(info.EgressRules) == 0 {
			lines = append(lines, OverlayWarningStyle.Render("  No egress rules = all egress denied"))
			lines = append(lines, "")
		}

		for i, rule := range info.EgressRules {
			lines = append(lines, OverlayNormalStyle.Render(fmt.Sprintf("  Rule %d:", i+1)))
			ruleLines := renderNetpolRuleDiagram(rule, targetLabel, false, width,
				boxBorderStyle, arrowStyle, labelStyle, cidrStyle, greenStyle)
			lines = append(lines, ruleLines...)
			lines = append(lines, "")
		}
	}

	// No rules at all.
	if !hasIngress && !hasEgress && len(info.IngressRules) == 0 && len(info.EgressRules) == 0 {
		lines = append(lines, OverlayDimStyle.Render("  No policy types or rules defined"))
		lines = append(lines, "")
	}

	// Footer.
	lines = append(lines, "")

	// Content area height: total height minus hint bar (1 line).
	maxVisible := max(height-1, 3)

	// Clamp scroll.
	maxScroll := max(len(lines)-maxVisible, 0)
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	end := min(scroll+maxVisible, len(lines))
	visible := lines[scroll:end]

	// Pad visible lines to fill the content area so the hint bar stays at the bottom.
	for len(visible) < maxVisible {
		visible = append(visible, "")
	}

	body := strings.Join(visible, "\n")

	// Styled hint bar matching the app's bottom bar pattern.
	scrollInfo := ""
	if maxScroll > 0 {
		scrollInfo = DimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, maxScroll+1))
	}
	hintParts := []string{
		HelpKeyStyle.Render("j/k") + DimStyle.Render(":scroll"),
		HelpKeyStyle.Render("g/G") + DimStyle.Render(":top/bottom"),
		HelpKeyStyle.Render("ctrl+d/u") + DimStyle.Render(":half page"),
		HelpKeyStyle.Render("ctrl+f/b") + DimStyle.Render(":page"),
		HelpKeyStyle.Render("esc") + DimStyle.Render(":close"),
	}
	hint := StatusBarBgStyle.Width(width).Render(strings.Join(hintParts, DimStyle.Render(" | ")) + scrollInfo)

	return body + "\n" + hint
}

// renderNetpolRuleDiagram renders a visual diagram for a single ingress/egress rule.
// For ingress: [Source] -----> [Target Pods]
// For egress:  [Target Pods] -----> [Destination]
func renderNetpolRuleDiagram(
	rule NetpolRuleEntry,
	targetLabel string,
	isIngress bool,
	width int,
	boxBorder, arrowSt, labelSt, cidrSt, greenSt lipgloss.Style,
) []string {
	var lines []string

	// Maximum label width for truncation (roughly half the available width).
	maxLabel := 0
	if width > 0 {
		maxLabel = width / 2
	}

	truncLabel := func(s string) string {
		if maxLabel > 0 && len(s) > maxLabel {
			return s[:maxLabel-1] + "~"
		}
		return s
	}

	for _, peer := range rule.Peers {
		// Build the peer box content.
		var peerLines []string
		switch peer.Type {
		case "All":
			peerLines = append(peerLines, greenSt.Render("All"))
		case "Pod":
			peerLines = append(peerLines, OverlayNormalStyle.Render("Pod:"))
			if len(peer.Selector) > 0 {
				peerKeys := make([]string, 0, len(peer.Selector))
				for k := range peer.Selector {
					peerKeys = append(peerKeys, k)
				}
				sort.Strings(peerKeys)
				for _, k := range peerKeys {
					peerLines = append(peerLines, labelSt.Render(truncLabel(k+"="+peer.Selector[k])))
				}
			} else {
				peerLines = append(peerLines, OverlayDimStyle.Render("(all pods)"))
			}
		case "Namespace":
			peerLines = append(peerLines, OverlayNormalStyle.Render("Namespace:"))
			peerLines = append(peerLines, labelSt.Render(truncLabel(peer.Namespace)))
		case "Namespace+Pod":
			peerLines = append(peerLines, OverlayNormalStyle.Render("NS: "+truncLabel(peer.Namespace)))
			if len(peer.Selector) > 0 {
				peerLines = append(peerLines, OverlayNormalStyle.Render("Pod:"))
				nsPodKeys := make([]string, 0, len(peer.Selector))
				for k := range peer.Selector {
					nsPodKeys = append(nsPodKeys, k)
				}
				sort.Strings(nsPodKeys)
				for _, k := range nsPodKeys {
					peerLines = append(peerLines, labelSt.Render(truncLabel(k+"="+peer.Selector[k])))
				}
			}
		case "CIDR":
			peerLines = append(peerLines, OverlayNormalStyle.Render("CIDR:"))
			peerLines = append(peerLines, cidrSt.Render(peer.CIDR))
			if len(peer.Except) > 0 {
				peerLines = append(peerLines, OverlayDimStyle.Render("Except:"))
				for _, e := range peer.Except {
					peerLines = append(peerLines, cidrSt.Render("  "+e))
				}
			}
		}

		// Add port info.
		if len(rule.Ports) > 0 {
			peerLines = append(peerLines, "")
			for _, port := range rule.Ports {
				portStr := port.Protocol
				if port.Port != "" {
					portStr += "/" + port.Port
				}
				peerLines = append(peerLines, OverlayDimStyle.Render("Port: ")+greenSt.Render(portStr))
			}
		}

		// Build the target box content.
		var targetLines []string
		targetLines = append(targetLines, greenSt.Render("Target Pods"))
		for _, line := range strings.Split(targetLabel, "\n") {
			targetLines = append(targetLines, labelSt.Render(truncLabel(line)))
		}

		// Render the two boxes with an arrow between them.
		var leftBox, rightBox []string
		var arrow string
		if isIngress {
			leftBox = peerLines
			rightBox = targetLines
			arrow = arrowSt.Render(" -----> ")
		} else {
			leftBox = targetLines
			rightBox = peerLines
			arrow = arrowSt.Render(" -----> ")
		}

		boxLines := renderTwoBoxes(leftBox, rightBox, arrow, boxBorder, width)
		for _, bl := range boxLines {
			lines = append(lines, "  "+bl)
		}
	}

	return lines
}

// renderTwoBoxes renders two boxes side by side connected by an arrow.
// Uses box-drawing characters for borders. If maxWidth > 0, box widths are
// capped so the total diagram fits within that width, and content lines are
// truncated accordingly.
func renderTwoBoxes(leftContent, rightContent []string, arrow string, borderStyle lipgloss.Style, maxWidth int) []string {
	// Calculate box widths.
	leftW := 0
	for _, line := range leftContent {
		if w := lipgloss.Width(line); w > leftW {
			leftW = w
		}
	}
	rightW := 0
	for _, line := range rightContent {
		if w := lipgloss.Width(line); w > rightW {
			rightW = w
		}
	}

	// Add padding.
	leftW += 2
	rightW += 2

	// Ensure minimum widths.
	if leftW < 14 {
		leftW = 14
	}
	if rightW < 14 {
		rightW = 14
	}

	arrowW := lipgloss.Width(arrow)

	// Cap box widths so the total diagram fits within maxWidth.
	// Total width = (1 + 1 + leftW + 1 + 1) + arrowW + (1 + 1 + rightW + 1 + 1)
	//             = leftW + 4 + arrowW + rightW + 4
	if maxWidth > 0 {
		// Overhead: left border(1) + space(1) + space(1) + right border(1) = 4 per box.
		overhead := 4 + arrowW + 4
		available := maxWidth - overhead
		if available < 2 {
			available = 2
		}
		if leftW+rightW > available {
			// Split available space proportionally, each gets at least half of minimum (7).
			half := available / 2
			if leftW > half && rightW > half {
				leftW = half
				rightW = available - half
			} else if leftW > half {
				leftW = available - rightW
			} else {
				rightW = available - leftW
			}
			if leftW < 7 {
				leftW = 7
			}
			if rightW < 7 {
				rightW = 7
			}
		}
	}

	// Equalize heights.
	maxH := max(len(leftContent), len(rightContent))
	for len(leftContent) < maxH {
		leftContent = append(leftContent, "")
	}
	for len(rightContent) < maxH {
		rightContent = append(rightContent, "")
	}

	// Truncate content lines that exceed their box width.
	for i, line := range leftContent {
		if lipgloss.Width(line) > leftW {
			leftContent[i] = ansi.Truncate(line, leftW, "~")
		}
	}
	for i, line := range rightContent {
		if lipgloss.Width(line) > rightW {
			rightContent[i] = ansi.Truncate(line, rightW, "~")
		}
	}

	var result []string

	// The inner width of each box is: 1 space + content + 1 space = leftW + 2.
	// Border dashes span that same inner width.
	leftInner := leftW + 2
	rightInner := rightW + 2

	// Top borders.
	topLine := borderStyle.Render("\u250c"+strings.Repeat("\u2500", leftInner)+"\u2510") +
		strings.Repeat(" ", arrowW) +
		borderStyle.Render("\u250c"+strings.Repeat("\u2500", rightInner)+"\u2510")
	result = append(result, topLine)

	// Content lines with arrow at the midpoint.
	midRow := maxH / 2
	for i := 0; i < maxH; i++ {
		leftLine := padRight(leftContent[i], leftW)
		rightLine := padRight(rightContent[i], rightW)

		connector := strings.Repeat(" ", arrowW)
		if i == midRow {
			connector = arrow
		}

		line := borderStyle.Render("\u2502") + " " + leftLine + " " + borderStyle.Render("\u2502") +
			connector +
			borderStyle.Render("\u2502") + " " + rightLine + " " + borderStyle.Render("\u2502")

		result = append(result, line)
	}

	// Bottom borders.
	bottomLine := borderStyle.Render("\u2514"+strings.Repeat("\u2500", leftInner)+"\u2518") +
		strings.Repeat(" ", arrowW) +
		borderStyle.Render("\u2514"+strings.Repeat("\u2500", rightInner)+"\u2518")
	result = append(result, bottomLine)

	return result
}

// diffLine classifies a line in the diff output.
type diffLine struct {
	left    string // content for left column ("" if absent)
	right   string // content for right column ("" if absent)
	status  byte   // '=' same, '<' only left, '>' only right, '~' both present but different
}

// computeDiff produces a line-by-line diff of two YAML texts using a simple
// longest-common-subsequence algorithm, then pairs up the differences.
func computeDiff(leftText, rightText string) []diffLine {
	leftLines := strings.Split(leftText, "\n")
	rightLines := strings.Split(rightText, "\n")

	// Remove trailing empty line from Split if the text ends with newline.
	if len(leftLines) > 0 && leftLines[len(leftLines)-1] == "" {
		leftLines = leftLines[:len(leftLines)-1]
	}
	if len(rightLines) > 0 && rightLines[len(rightLines)-1] == "" {
		rightLines = rightLines[:len(rightLines)-1]
	}

	n := len(leftLines)
	m := len(rightLines)

	// Build LCS table.
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if leftLines[i-1] == rightLines[j-1] {
				dp[i][j] = dp[i-1][j-1] + 1
			} else if dp[i-1][j] >= dp[i][j-1] {
				dp[i][j] = dp[i-1][j]
			} else {
				dp[i][j] = dp[i][j-1]
			}
		}
	}

	// Backtrack to produce the diff.
	var result []diffLine
	i, j := n, m
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && leftLines[i-1] == rightLines[j-1] {
			result = append(result, diffLine{left: leftLines[i-1], right: rightLines[j-1], status: '='})
			i--
			j--
		} else if j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]) {
			result = append(result, diffLine{right: rightLines[j-1], status: '>'})
			j--
		} else {
			result = append(result, diffLine{left: leftLines[i-1], status: '<'})
			i--
		}
	}

	// Reverse the result (backtracking produces it in reverse order).
	for lo, hi := 0, len(result)-1; lo < hi; lo, hi = lo+1, hi-1 {
		result[lo], result[hi] = result[hi], result[lo]
	}

	return result
}

// DiffViewTotalLines returns the total number of rendered lines for a diff view,
// used to calculate scroll bounds.
func DiffViewTotalLines(left, right string) int {
	lines := computeDiff(left, right)
	// +1 for the header line.
	return len(lines) + 1
}

// RenderDiffView renders a side-by-side YAML diff view.
// Lines present only on the left are shown in red, only on the right in green,
// and lines that match are shown normally.
func RenderDiffView(left, right, leftName, rightName string, scroll, width, height int, lineNumbers bool) string {
	diffLines := computeDiff(left, right)

	// Styles for diff highlighting.
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))
	headerNameStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true)
	separatorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))

	// Line number gutter width.
	var gutterWidth int
	if lineNumbers {
		gutterWidth = len(fmt.Sprintf("%d", len(diffLines))) + 1 // digits + space
	}

	// Calculate column widths: split the available content area in half with a separator.
	// Available content width = width - 4 (border left/right + padding left/right).
	colWidth := (width - 7 - gutterWidth*2) / 2 // 4 = border+padding, 3 = " | " separator, gutter on each side
	if colWidth < 10 {
		colWidth = 10
	}

	// Build header.
	gutterPad := strings.Repeat(" ", gutterWidth)
	leftHeader := headerNameStyle.Render(truncateToWidth(leftName, colWidth))
	rightHeader := headerNameStyle.Render(truncateToWidth(rightName, colWidth))
	header := gutterPad + padToWidth(leftHeader, colWidth) + separatorStyle.Render(" | ") + gutterPad + padToWidth(rightHeader, colWidth)

	// Reserve lines for title, hint bar, border (top+bottom), header, and separator.
	maxLines := height - 6
	if maxLines < 3 {
		maxLines = 3
	}

	// Clamp scroll.
	totalLines := len(diffLines)
	maxScroll := totalLines - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Visible slice.
	visible := diffLines[scroll:]
	if len(visible) > maxLines {
		visible = visible[:maxLines]
	}

	// Track left/right line numbers independently.
	leftNum, rightNum := 1, 1
	for i := 0; i < scroll && i < len(diffLines); i++ {
		switch diffLines[i].status {
		case '=':
			leftNum++
			rightNum++
		case '<':
			leftNum++
		case '>':
			rightNum++
		}
	}

	var rows []string
	for _, dl := range visible {
		var leftCol, rightCol, leftGutter, rightGutter string
		switch dl.status {
		case '=':
			leftCol = normalStyle.Render(truncateToWidth(dl.left, colWidth))
			rightCol = normalStyle.Render(truncateToWidth(dl.right, colWidth))
			if lineNumbers {
				leftGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, leftNum))
				rightGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, rightNum))
			}
			leftNum++
			rightNum++
		case '<':
			leftCol = removedStyle.Render(truncateToWidth(dl.left, colWidth))
			rightCol = ""
			if lineNumbers {
				leftGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, leftNum))
				rightGutter = strings.Repeat(" ", gutterWidth)
			}
			leftNum++
		case '>':
			leftCol = ""
			rightCol = addedStyle.Render(truncateToWidth(dl.right, colWidth))
			if lineNumbers {
				leftGutter = strings.Repeat(" ", gutterWidth)
				rightGutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, rightNum))
			}
			rightNum++
		}
		row := leftGutter + padToWidth(leftCol, colWidth) + separatorStyle.Render(" | ") + rightGutter + padToWidth(rightCol, colWidth)
		rows = append(rows, row)
	}

	// Pad rows to fill available height so content fills the border.
	for len(rows) < maxLines {
		rows = append(rows, "")
	}

	title := TitleStyle.Render("Resource Diff")
	sepLine := gutterPad + strings.Repeat("-", colWidth) + " + " + gutterPad + strings.Repeat("-", colWidth)
	bodyContent := header + "\n" + separatorStyle.Render(sepLine) + "\n" + strings.Join(rows, "\n")

	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Padding(0, 1).
		Width(width - 2).
		Height(maxLines + 2) // +2 for header + separator lines
	body := borderStyle.Render(bodyContent)

	// Hint bar.
	hintParts := []string{
		HelpKeyStyle.Render("j/k") + DimStyle.Render(": scroll"),
		HelpKeyStyle.Render("g/G") + DimStyle.Render(": top/bottom"),
		HelpKeyStyle.Render("ctrl+d/u") + DimStyle.Render(": half page"),
		HelpKeyStyle.Render("ctrl+f/b") + DimStyle.Render(": page"),
		HelpKeyStyle.Render("l") + DimStyle.Render(": lines"),
		HelpKeyStyle.Render("u") + DimStyle.Render(": unified"),
		HelpKeyStyle.Render("q/esc") + DimStyle.Render(": back"),
	}
	scrollInfo := DimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, max(1, maxScroll+1)))
	hint := StatusBarBgStyle.Width(width).Render(strings.Join(hintParts, DimStyle.Render(" | ")) + scrollInfo)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

// RenderUnifiedDiffView renders a unified diff view of two YAML resources.
func RenderUnifiedDiffView(left, right, leftName, rightName string, scroll, width, height int, lineNumbers bool) string {
	diffLines := computeDiff(left, right)

	// Styles.
	removedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	addedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary))
	normalStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFile))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Bold(true)

	// Line number gutter width (for the unified line number).
	var gutterWidth int
	if lineNumbers {
		gutterWidth = len(fmt.Sprintf("%d", len(diffLines)+2)) + 1 // +2 for header lines
	}

	// Build unified diff lines.
	var lines []string
	lines = append(lines, headerStyle.Render("--- "+leftName))
	lines = append(lines, headerStyle.Render("+++ "+rightName))

	lineNum := 1
	for _, dl := range diffLines {
		var gutter string
		if lineNumbers {
			gutter = DimStyle.Render(fmt.Sprintf("%*d ", gutterWidth-1, lineNum))
		}
		lineNum++
		switch dl.status {
		case '=':
			lines = append(lines, gutter+normalStyle.Render(" "+dl.left))
		case '<':
			lines = append(lines, gutter+removedStyle.Render("-"+dl.left))
		case '>':
			lines = append(lines, gutter+addedStyle.Render("+"+dl.right))
		}
	}

	title := TitleStyle.Render("Resource Diff (unified)")

	// Reserve lines for title, hint bar, and border (top+bottom).
	maxLines := height - 4
	if maxLines < 3 {
		maxLines = 3
	}

	// Clamp scroll.
	totalLines := len(lines)
	maxScroll := totalLines - maxLines
	if maxScroll < 0 {
		maxScroll = 0
	}
	if scroll > maxScroll {
		scroll = maxScroll
	}
	if scroll < 0 {
		scroll = 0
	}

	// Visible slice.
	visible := lines[scroll:]
	if len(visible) > maxLines {
		visible = visible[:maxLines]
	}

	// Pad to fill available height so content fills the border.
	for len(visible) < maxLines {
		visible = append(visible, "")
	}

	bodyContent := strings.Join(visible, "\n")
	borderStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorPrimary)).
		Padding(0, 1).
		Width(width - 2).
		Height(maxLines)
	body := borderStyle.Render(bodyContent)

	// Hint bar.
	hintParts := []string{
		HelpKeyStyle.Render("j/k") + DimStyle.Render(": scroll"),
		HelpKeyStyle.Render("g/G") + DimStyle.Render(": top/bottom"),
		HelpKeyStyle.Render("ctrl+d/u") + DimStyle.Render(": half page"),
		HelpKeyStyle.Render("ctrl+f/b") + DimStyle.Render(": page"),
		HelpKeyStyle.Render("l") + DimStyle.Render(": lines"),
		HelpKeyStyle.Render("u") + DimStyle.Render(": side-by-side"),
		HelpKeyStyle.Render("q/esc") + DimStyle.Render(": back"),
	}
	scrollInfo := DimStyle.Render(fmt.Sprintf(" [%d/%d]", scroll+1, max(1, maxScroll+1)))
	hint := StatusBarBgStyle.Width(width).Render(strings.Join(hintParts, DimStyle.Render(" | ")) + scrollInfo)

	return lipgloss.JoinVertical(lipgloss.Left, title, body, hint)
}

// UnifiedDiffViewTotalLines returns the total number of rendered lines for a unified diff view.
func UnifiedDiffViewTotalLines(left, right string) int {
	lines := computeDiff(left, right)
	// +2 for the --- and +++ header lines.
	return len(lines) + 2
}

// truncateToWidth truncates a string to fit within the given visual width.
func truncateToWidth(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	// Progressively truncate until it fits.
	runes := []rune(s)
	for len(runes) > 0 {
		runes = runes[:len(runes)-1]
		if lipgloss.Width(string(runes)) <= maxWidth-1 {
			return string(runes) + "~"
		}
	}
	return ""
}

// padToWidth pads a styled string with spaces to reach the desired visual width.
func padToWidth(s string, targetWidth int) string {
	w := lipgloss.Width(s)
	if w >= targetWidth {
		return s
	}
	return s + strings.Repeat(" ", targetWidth-w)
}

// PlaceOverlay centers an overlay on top of a background by building a full-screen
// layer with the overlay content padded to the correct position.
func PlaceOverlay(width, height int, overlay, background string) string {
	bgLines := strings.Split(background, "\n")

	// Ensure background has enough lines.
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	// Trim excess.
	if len(bgLines) > height {
		bgLines = bgLines[:height]
	}

	ovLines := strings.Split(overlay, "\n")
	ovHeight := len(ovLines)
	ovWidth := 0
	for _, line := range ovLines {
		if w := lipgloss.Width(line); w > ovWidth {
			ovWidth = w
		}
	}

	startRow := (height - ovHeight) / 2
	startCol := (width - ovWidth) / 2
	if startRow < 0 {
		startRow = 0
	}
	if startCol < 0 {
		startCol = 0
	}

	// Build result: replace background lines in the overlay region.
	result := make([]string, len(bgLines))
	copy(result, bgLines)

	// Ensure each background line is at least full width so the overlay
	// doesn't leave gaps when the background has short lines.
	for i, line := range result {
		lineW := lipgloss.Width(line)
		if lineW < width {
			result[i] = line + strings.Repeat(" ", width-lineW)
		}
	}

	for i, ovLine := range ovLines {
		row := startRow + i
		if row >= len(result) {
			break
		}
		bgLine := result[row]
		ovVisualWidth := lipgloss.Width(ovLine)
		leftBg := ansi.Truncate(bgLine, startCol, "")
		rightBg := ansi.TruncateLeft(bgLine, startCol+ovVisualWidth, "")
		result[row] = leftBg + ovLine + rightBg
	}

	return strings.Join(result, "\n")
}
