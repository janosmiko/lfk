package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
)

// sortIndicatorForColumn returns a sort direction indicator (" ▲" or " ▼") if
// the given column name matches the currently sorted column, or "" otherwise.
// sortIndicatorForColumn returns "↑" or "↓" if the given column is sorted, or "".
func sortIndicatorForColumn(colName string) string {
	if ActiveSortColumnName == colName {
		if ActiveSortAscending {
			return "\u2191" // ↑
		}
		return "\u2193" // ↓
	}
	return ""
}

// headerWithIndicator returns a column header string that fits within colWidth,
// with the sort indicator placed at the end using the column's padding space.
func headerWithIndicator(label string, colName string, colWidth int) string {
	ind := sortIndicatorForColumn(colName)
	if ind == "" {
		return padRight(label, colWidth)
	}
	// Truncate label to make room for the indicator.
	maxLabel := colWidth - 2 // space + indicator
	if maxLabel < 1 {
		maxLabel = 1
	}
	if len(label) > maxLabel {
		label = label[:maxLabel]
	}
	return padRight(label+" "+ind, colWidth)
}

// extraColumn represents an additional column discovered from item data.
type extraColumn struct {
	key      string // column key (e.g., "IP", "Node")
	width    int    // display width for this column
	hasArrow bool   // true if any value in this column has a trend arrow
}

// ExtraColumnInfo is an exported representation of an extra column for use by
// the app layer (e.g., header click handling).
type ExtraColumnInfo struct {
	Key   string
	Width int
}

// CollectExtraColumns is an exported wrapper around collectExtraColumns.
// It returns the extra columns as ExtraColumnInfo for use outside the ui package.
func CollectExtraColumns(items []model.Item, totalWidth, usedWidth int, kind string) []ExtraColumnInfo {
	cols := collectExtraColumns(items, totalWidth, usedWidth, kind)
	result := make([]ExtraColumnInfo, len(cols))
	for i, c := range cols {
		result[i] = ExtraColumnInfo{Key: c.key, Width: c.width}
	}
	return result
}

// ActiveSessionColumns holds the session-only column override for the current
// resource type. Set by the app before rendering. Nil means no override.
var ActiveSessionColumns []string

// ActiveHiddenBuiltinColumns holds the set of built-in column keys that should
// be suppressed in the current middle-column render. Valid keys: "Namespace",
// "Ready", "Restarts", "Age", "Status". Set by the app before rendering.
// Nil means no overrides.
var ActiveHiddenBuiltinColumns map[string]bool

// collectExtraColumns discovers which extra columns to show based on item data and config.
// usedWidth is the width already consumed by fixed columns (excluding name).
// kind is the resource Kind (e.g. "Pod") used to resolve per-type column overrides.
// colInfo tracks metadata about a single extra column during collection.
type colInfo struct {
	key      string
	maxValW  int
	count    int
	hasArrow bool // true if any value in this column has a trend arrow
}

func collectExtraColumns(items []model.Item, totalWidth, usedWidth int, kind string) []extraColumn {
	// Collect all available column keys and their max value widths.
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

	candidates := selectColumnCandidates(seen, order, kind, items)

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
			colW = maxColW
		}
		colW++ // spacing
		if colW > remainingW {
			break
		}
		result = append(result, extraColumn{key: key, width: colW, hasArrow: info.hasArrow})
		remainingW -= colW
	}

	// Remaining width is not assigned to extra columns — it flows back to the
	// NAME column via the caller's width calculation, keeping resource names
	// readable instead of padding the last extra column.

	return result
}

// selectColumnCandidates determines which extra columns to display based on
// session overrides, per-kind config, or auto-detection.
//
// ActiveSessionColumns is the authoritative signal when non-nil: an empty
// slice means the user explicitly configured this kind with no extras and
// must not fall through to auto-detect. Only a nil slice means "no session
// override" and lets the config / auto-detect paths run.
func selectColumnCandidates(seen map[string]*colInfo, order []string, kind string, items []model.Item) []string {
	if ActiveSessionColumns != nil {
		candidates := make([]string, 0, len(ActiveSessionColumns))
		for _, key := range ActiveSessionColumns {
			if _, ok := seen[key]; ok {
				candidates = append(candidates, key)
			}
		}
		return candidates
	}

	configCols := ColumnsForKind(kind, ActiveContext)
	if len(configCols) > 0 {
		if len(configCols) == 1 && configCols[0] == "*" {
			return order
		}
		var candidates []string
		for _, cfgKey := range configCols {
			if _, ok := seen[cfgKey]; ok {
				candidates = append(candidates, cfgKey)
			}
		}
		return candidates
	}

	return autoDetectColumns(seen, order, items)
}

// autoDetectColumns selects columns based on heuristic thresholds and blocked lists.
func autoDetectColumns(seen map[string]*colInfo, order []string, items []model.Item) []string {
	blocked := blockedColumnsForMode()
	// Raw metrics columns are always blocked.
	for _, k := range []string{"CPU Req", "CPU Lim", "Mem Req", "Mem Lim", "CPU Alloc", "Mem Alloc"} {
		blocked[k] = true
	}

	threshold := len(items) / 5
	if threshold < 1 {
		threshold = 1
	}
	alwaysShow := map[string]bool{"Condition": true}
	var candidates []string
	for _, key := range order {
		if blocked[key] || isHiddenColumnPrefix(key) {
			continue
		}
		info := seen[key]
		if info.count >= threshold || alwaysShow[key] {
			candidates = append(candidates, key)
		}
	}
	return candidates
}

// isHiddenColumnPrefix returns true if the column key uses a prefix reserved for internal data.
func isHiddenColumnPrefix(key string) bool {
	return strings.HasPrefix(key, "__") ||
		strings.HasPrefix(key, "secret:") ||
		strings.HasPrefix(key, "owner:") ||
		strings.HasPrefix(key, "data:") ||
		strings.HasPrefix(key, "condition:") ||
		strings.HasPrefix(key, "step:")
}

// blockedColumnsForMode returns the set of columns blocked in the current display mode.
func blockedColumnsForMode() map[string]bool {
	if ActiveFullscreenMode {
		return map[string]bool{
			"Health Message": true, "Keys": true,
			"Service Account": true, "Images": true, "Image": true,
			"Health": true, "Sync": true, "Path": true,
			"Labels": true, "Finalizers": true, "Annotations": true,
			"Used By": true, "Deletion": true, "Selector": true,
		}
	}
	return map[string]bool{
		"IP": true, "Images": true, "Image": true,
		"Host IP": true, "Pod IP": true, "Cluster IP": true,
		"Repo": true, "Path": true, "Dest Server": true,
		"Health Message": true, "Keys": true,
		"Service Account": true, "Node": true,
		"QoS": true, "Priority Class": true,
		"Health": true, "Sync": true, "Dest NS": true,
		"Sync Message": true, "Sync Errors": true,
		"OS": true, "Runtime": true,
		"Hostname": true, "InternalIP": true, "ExternalIP": true,
		"Source": true,
		"Labels": true, "Finalizers": true, "Annotations": true,
		"Used By": true, "Deletion": true, "Selector": true,
	}
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

// plainExtraCell builds the plain-text cell for a single extra column.
// When item is nil, the cell renders a header value (uppercased key plus
// sort indicator).
func plainExtraCell(ec extraColumn, item *model.Item) string {
	var val string
	if item == nil {
		val = strings.ToUpper(ec.key) + sortIndicatorForColumn(ec.key)
	} else {
		val = getExtraColumnValue(item, ec.key)
	}
	switch {
	case strings.HasPrefix(val, "↑ ") || strings.HasPrefix(val, "↓ "):
		arrow := string([]rune(val)[0])
		baseVal := val[len("↑ "):]
		return arrow + padRight(Truncate(baseVal, ec.width-2), ec.width-1)
	case ec.hasArrow:
		return " " + padRight(Truncate(val, ec.width-2), ec.width-1)
	default:
		return padRight(Truncate(val, ec.width-1), ec.width)
	}
}

// styledExtraCell builds the styled cell for a single extra column.
func styledExtraCell(ec extraColumn, item *model.Item) string {
	val := getExtraColumnValue(item, ec.key)
	style := resourceColumnStyle(ec.key, val)
	switch {
	case strings.HasPrefix(val, "↑ "):
		baseVal := val[len("↑ "):]
		return ErrorStyle.Render("↑") + style.Render(padRight(Truncate(baseVal, ec.width-2), ec.width-1))
	case strings.HasPrefix(val, "↓ "):
		baseVal := val[len("↓ "):]
		return StatusRunning.Render("↓") + style.Render(padRight(Truncate(baseVal, ec.width-2), ec.width-1))
	case ec.hasArrow:
		return NormalStyle.Render(" ") + style.Render(padRight(Truncate(val, ec.width-2), ec.width-1))
	default:
		return style.Render(padRight(Truncate(val, ec.width-1), ec.width))
	}
}

// plainBuiltinCell builds the plain-text cell for a single built-in column.
// Values are the already-resolved display strings for this row (e.g. ns with
// dash fallback, preprocessed restarts with arrow prefix).
func plainBuiltinCell(key string, ns, ready, restarts, status, age string,
	nsW, readyW, restartsW, statusW, ageW int,
) string {
	switch key {
	case "Namespace":
		return padRight(Truncate(ns, nsW-1), nsW)
	case "Ready":
		return padRight(ready, readyW)
	case "Restarts":
		return padRight(restarts, restartsW)
	case "Status":
		return padRight(Truncate(status, statusW-1), statusW)
	case "Age":
		return padRight(age, ageW)
	}
	return ""
}

// styledBuiltinCell builds the styled cell for a single built-in column.
// Namespaces are dimmed, Ready is dimmed, Restarts is delegated to
// styledRestartsCell for its arrow handling, Status and Age use their own
// status-aware style helpers.
func styledBuiltinCell(key string, item model.Item,
	nsW, readyW, restartsW, statusW, ageW int, anyRecentRestart bool,
) string {
	switch key {
	case "Namespace":
		ns := item.Namespace
		if ns == "" {
			ns = "-"
		}
		return DimStyle.Render(padRight(Truncate(ns, nsW-1), nsW))
	case "Ready":
		return DimStyle.Render(padRight(item.Ready, readyW))
	case "Restarts":
		return styledRestartsCell(item, restartsW, anyRecentRestart)
	case "Status":
		return StatusStyle(item.Status).Render(padRight(Truncate(item.Status, statusW-1), statusW))
	case "Age":
		return AgeStyle(item.Age).Render(padRight(item.Age, ageW))
	}
	return ""
}

// styledRestartsCell renders the restarts column with recent-restart arrow
// styling. Rows whose LastRestartAt is within the past hour are tagged with
// an up-arrow; when any row in the table has a recent restart, rows without
// one get a space prefix so values remain column-aligned.
func styledRestartsCell(item model.Item, restartsW int, anyRecentRestart bool) string {
	restartCount, _ := strconv.Atoi(item.Restarts)
	recentRestart := !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour
	switch {
	case restartCount > 0 && recentRestart:
		restartText := "↑" + item.Restarts
		if restartCount >= 5 {
			return ErrorStyle.Render(padRight(restartText, restartsW))
		}
		return StatusFailed.Render(padRight(restartText, restartsW))
	case anyRecentRestart:
		return DimStyle.Render(padRight(" "+item.Restarts, restartsW))
	default:
		return DimStyle.Render(padRight(item.Restarts, restartsW))
	}
}

// formatTableRowOrdered builds a plain-text table row using the given column
// order. Name is always emitted first. The preprocessed values (ns, ready,
// restarts, status, age) are passed through since they have row-specific
// handling upstream (e.g. the cursor row preprocesses restarts for arrow
// alignment).
func formatTableRowOrdered(name, ns, ready, restarts, status, age string,
	nameW, nsW, readyW, restartsW, statusW, ageW int,
	order []string, extraCols []extraColumn, item *model.Item,
) string {
	row := padRight(Truncate(name, nameW-1), nameW)
	for _, key := range order {
		if isBuiltinColumnKey(key) {
			row += plainBuiltinCell(key, ns, ready, restarts, status, age,
				nsW, readyW, restartsW, statusW, ageW)
			continue
		}
		// Extra column: look up metadata and emit via plainExtraCell.
		for _, ec := range extraCols {
			if ec.key == key {
				row += plainExtraCell(ec, item)
				break
			}
		}
	}
	return row
}

// formatTableRowStyledOrdered builds a styled table row using the given
// column order. Name (with icon handling) is always emitted first via the
// existing styled name helper; the rest is dispatched per-key.
func formatTableRowStyledOrdered(item model.Item,
	nameW, nsW, readyW, restartsW, statusW, ageW int,
	order []string, extraCols []extraColumn, anyRecentRestart bool,
) string {
	base := styledNameCell(item, nameW)
	for _, key := range order {
		if isBuiltinColumnKey(key) {
			base += styledBuiltinCell(key, item, nsW, readyW, restartsW, statusW, ageW, anyRecentRestart)
			continue
		}
		for _, ec := range extraCols {
			if ec.key == key {
				base += styledExtraCell(ec, &item)
				break
			}
		}
	}
	return base
}

// styledNameCell renders the Name column with optional icon and dimmed
// styling for completed items. Pods in Succeeded or Completed status get
// their name dimmed; otherwise NormalStyle is used. The active highlight
// query is applied to the resolved display name.
func styledNameCell(item model.Item, nameW int) string {
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
		nameRemaining := nameW - iconVisualW - 1 // -1 reserves gap before next column
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
		return icon + namePart + strings.Repeat(" ", pad)
	}
	displayName := Truncate(item.Name, nameW-1)
	if ActiveHighlightQuery != "" {
		displayName = highlightName(displayName, ActiveHighlightQuery)
	}
	return nameStyle.Render(padRight(displayName, nameW))
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(BaseBg)
	case "Last Sync", "Health", "Sync", "Reason":
		return StatusStyle(val)
	case "Synced At":
		if strings.HasPrefix(val, "syncing") {
			return StatusProgressing // blue: sync in progress
		}
		return DimStyle
	case "AutoSync":
		switch {
		case val == "On/SH/P":
			return StatusRunning // green: fully enabled
		case strings.HasPrefix(val, "On"):
			return StatusProgressing // blue: partially enabled
		default:
			return StatusFailed // red: disabled
		}
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
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(BaseBg)
	case pct >= 75:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorOrange)).Bold(true).Background(BaseBg)
	default:
		return DimStyle
	}
}

// ParseResourceValue parses a CPU (millicores) or memory (bytes) string back to int64.
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
	// Use lipgloss.Width to measure the visual width, which correctly
	// ignores ANSI escape sequences in styled text.
	if lipgloss.Width(s) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "~"
	}
	// Strip ANSI codes, truncate the visible content, then append the marker.
	// This avoids cutting in the middle of an escape sequence.
	runes := []rune(ansi.Strip(s))
	if len(runes) <= maxW {
		return s
	}
	return string(runes[:maxW-1]) + "~"
}

// truncateNoMarker truncates a string to maxW runes without appending any marker.
// Used for wrappable columns where the remaining content continues on the next line.
func truncateNoMarker(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= maxW {
		return s
	}
	return string(runes[:maxW])
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
		Background(BarBg).
		Padding(0, 1)
	separatorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBorder)).
		Background(BarBg)
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
		tabContent := " " + strings.Join(parts, "")
		return lipgloss.NewStyle().Background(BarBg).Width(width).MaxWidth(width).Render(tabContent)
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

	tabContent := " " + strings.Join(parts, "")
	return lipgloss.NewStyle().Background(BarBg).Width(width).MaxWidth(width).Render(tabContent)
}
