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
	maxLabel := max(
		// space + indicator
		colWidth-2, 1)
	if len(label) > maxLabel {
		label = label[:maxLabel]
	}
	return padRight(label+" "+ind, colWidth)
}

// plainExtraCell builds the plain-text cell for a single extra column.
// When item is nil, the cell renders a header value (uppercased key plus
// sort indicator).
func plainExtraCell(ec extraColumn, item *model.Item) string {
	var val string
	if item == nil {
		val = columnHeaderLabel(ec.key) + sortIndicatorForColumn(ec.key)
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
// statusAbbreviations maps long-form Pod-ish status strings to a compact
// label used when the STATUS column has been shrunk under width pressure.
// Entries here are status values that are otherwise too verbose for narrow
// layouts; status values not in the map render as-is and rely on the
// width-aware Truncate fallback. AbbreviateStatusForWidth picks between
// the full string and the abbreviation based on the column's width budget.
var statusAbbreviations = map[string]string{
	"PodInitializing":            "Init",
	"ContainerCreating":          "Creating",
	"Terminating":                "Term",
	"CrashLoopBackOff":           "CrashLoop",
	"ImagePullBackOff":           "ImgPull",
	"ErrImagePull":               "ImgPull",
	"InvalidImageName":           "BadImage",
	"CreateContainerConfigError": "CfgErr",
	"CreateContainerError":       "CtrErr",
	"Succeeded":                  "Done",
	"Completed":                  "Done",
}

// AbbreviateStatusForWidth returns a status label that fits within w
// visible columns. Returns the full status when it already fits; otherwise
// looks up a curated abbreviation; otherwise falls back to the original
// (the caller will then truncate it). Pure function so the layout pass
// and the cell renderer can both use it.
func AbbreviateStatusForWidth(status string, w int) string {
	if len(status) <= w {
		return status
	}
	if abbrev, ok := statusAbbreviations[status]; ok {
		return abbrev
	}
	return status
}

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
		return padRight(Truncate(AbbreviateStatusForWidth(status, statusW-1), statusW-1), statusW)
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
		val := AbbreviateStatusForWidth(item.Status, statusW-1)
		return StatusStyle(val).Render(padRight(Truncate(val, statusW-1), statusW))
	case "Age":
		age := LiveAge(item)
		return AgeStyle(age).Render(padRight(age, ageW))
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
	var row strings.Builder
	row.WriteString(padRight(Truncate(name, nameW-1), nameW))
	for _, key := range order {
		if isBuiltinColumnKey(key) {
			row.WriteString(plainBuiltinCell(key, ns, ready, restarts, status, age,
				nsW, readyW, restartsW, statusW, ageW))
			continue
		}
		// Extra column: look up metadata and emit via plainExtraCell.
		for _, ec := range extraCols {
			if ec.key == key {
				row.WriteString(plainExtraCell(ec, item))
				break
			}
		}
	}
	return row.String()
}

// formatTableRowStyledOrdered builds a styled table row using the given
// column order. Name (with icon handling) is always emitted first via the
// existing styled name helper; the rest is dispatched per-key.
func formatTableRowStyledOrdered(item model.Item,
	nameW, nsW, readyW, restartsW, statusW, ageW int,
	order []string, extraCols []extraColumn, anyRecentRestart bool,
) string {
	var base strings.Builder
	base.WriteString(styledNameCell(item, nameW))
	for _, key := range order {
		if isBuiltinColumnKey(key) {
			base.WriteString(styledBuiltinCell(key, item, nsW, readyW, restartsW, statusW, ageW, anyRecentRestart))
			continue
		}
		for _, ec := range extraCols {
			if ec.key == key {
				base.WriteString(styledExtraCell(ec, &item))
				break
			}
		}
	}
	return base.String()
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
		nameRemaining := max(
			// -1 reserves gap before next column
			nameW-iconVisualW-1, 1)
		namePart := Truncate(item.Name, nameRemaining)
		if ActiveHighlightQuery != "" {
			namePart = highlightName(namePart, ActiveHighlightQuery)
		}
		nameVisualW := lipgloss.Width(namePart)
		pad := max(nameW-iconVisualW-nameVisualW, 0)
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
		if before, ok := strings.CutSuffix(val, "m"); ok {
			n, _ := strconv.ParseFloat(before, 64)
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

// Truncate truncates a string to maxW visual columns, appending "~" if
// truncated. ANSI escape sequences are preserved so styled text keeps its
// foreground/background colors when shortened — `ansi.Truncate` is grapheme-
// and width-aware and never cuts inside an escape sequence.
func Truncate(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= maxW {
		return s
	}
	if maxW <= 1 {
		return "~"
	}
	return ansi.Truncate(s, maxW-1, "") + "~"
}

// TruncateWithSuffix truncates body so that body + suffix fits within maxW
// visual columns, then right-pads with spaces so the suffix lands flush
// against the right edge. Empty suffix degrades to plain Truncate.
//
// Used by the cluster picker to render the per-row colour swatch at the
// end of the line: putting it before the name added a leading-space gap
// that made uncoloured rows look ragged. With the swatch as a suffix the
// name column stays aligned and the colour still ends up in a consistent,
// scannable position.
func TruncateWithSuffix(body, suffix string, maxW int) string {
	if suffix == "" {
		return Truncate(body, maxW)
	}
	if maxW <= 0 {
		return ""
	}
	suffixW := lipgloss.Width(suffix)
	if suffixW >= maxW {
		// Suffix would consume the entire row — drop the body and just
		// truncate the suffix so we don't accidentally hide the colour.
		return Truncate(suffix, maxW)
	}
	// Reserve room for the suffix plus one space of separation from the
	// body so the swatch isn't visually glued to the name.
	bodyMaxW := max(maxW-suffixW-1, 1)
	truncated := Truncate(body, bodyMaxW)
	pad := max(maxW-lipgloss.Width(truncated)-suffixW, 1)
	return truncated + strings.Repeat(" ", pad) + suffix
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
	maxLabelLen := max(maxBarW/max(1, len(tabLabels)), 8)

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

	// Reserve space for the leading " " padding (added below) and for the
	// arrow indicators that get prepended/appended once the window is
	// chosen. Without this reservation, the rendered tabContent can exceed
	// `width` and the outer Width(width).MaxWidth(width).Render call wraps
	// it to a second line, which hides the title bar above. Indicators are
	// only needed when we can't reach the corresponding edge from the
	// active tab, so we reserve their width conditionally to avoid dropping
	// tabs from the window unnecessarily.
	const leadingPadW = 1
	leftIndicatorW := lipgloss.Width(inactiveStyle.Render("◂")) + sepW
	rightIndicatorW := sepW + lipgloss.Width(inactiveStyle.Render("▸"))
	budget := maxBarW - leadingPadW
	if activeTab > 0 {
		budget -= leftIndicatorW
	}
	if activeTab < len(tabs)-1 {
		budget -= rightIndicatorW
	}
	// Always allow the active tab to render, even if reservations leave
	// almost nothing — lipgloss will clip if it's still wider than the bar.
	if budget < tabs[activeTab].width {
		budget = tabs[activeTab].width
	}

	// Window around active tab.
	left := activeTab
	right := activeTab
	usedW := tabs[activeTab].width

	for {
		expanded := false
		if left > 0 {
			needed := sepW + tabs[left-1].width
			if usedW+needed <= budget {
				left--
				usedW += needed
				expanded = true
			}
		}
		if right < len(tabs)-1 {
			needed := sepW + tabs[right+1].width
			if usedW+needed <= budget {
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
