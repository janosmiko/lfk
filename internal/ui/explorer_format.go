package ui

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
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
	// The indicator is one column wide and sits in the column's trailing
	// spacing — directly after the label, no separating space. Only truncate
	// the label when it genuinely doesn't fit, so a tight column like "RS"
	// keeps both letters ("RS↑") instead of dropping one ("R ↑").
	maxLabel := max(colWidth-1, 1)
	if len(label) > maxLabel {
		label = label[:maxLabel]
	}
	return padRight(label+ind, colWidth)
}

// headerSegment is one cell of the table header: its pre-padded plain text and
// the column key it represents. colName is empty for non-column padding (the
// selection marker and the union tile gutter), which never matches the active
// sort column.
type headerSegment struct {
	text    string
	colName string
}

// renderStyledHeader renders the header segments into a single line capped at
// width. The segment whose colName matches the active sort column is rendered
// with SortActiveHeaderStyle (accent) so the sorted column stands out; every
// other segment is dim+bold, matching the previous flat header. Truncation
// happens on each segment's plain text before styling, so no ANSI escape
// sequence is ever cut and styled cells stay self-contained.
func renderStyledHeader(segments []headerSegment, width int) string {
	var b strings.Builder
	dimStyle := DimStyle.Bold(true) // derive once; reused for every inactive cell
	remaining := width
	for _, seg := range segments {
		if remaining <= 0 {
			break
		}
		text := seg.text
		if lipgloss.Width(text) > remaining {
			text = Truncate(text, remaining)
		}
		remaining -= lipgloss.Width(text)
		if seg.colName != "" && seg.colName == ActiveSortColumnName {
			b.WriteString(SortActiveHeaderStyle.Render(text))
		} else {
			b.WriteString(dimStyle.Render(text))
		}
	}
	return b.String()
}

// plainExtraCell builds the plain-text cell for a single extra column.
// When item is nil, the cell renders a header value (uppercased key plus
// sort indicator).
func plainExtraCell(ec extraColumn, item *model.Item) string {
	var val string
	if item == nil {
		val = ColumnHeaderLabel(ec.key) + sortIndicatorForColumn(ec.key)
	} else {
		val = GetExtraColumnValue(item, ec.key)
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
	val := GetExtraColumnValue(item, ec.key)
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

// plainRestartsCell preprocesses the restarts value for plain-cell rows
// (cursor row, tinted rows): an up-arrow tags the item's own recent restart,
// and a space keeps alignment when any other row has one.
func plainRestartsCell(item model.Item, anyRecentRestart bool) string {
	restartCount, _ := strconv.Atoi(item.Restarts)
	recentRestart := !item.LastRestartAt.IsZero() && time.Since(item.LastRestartAt) < time.Hour
	switch {
	case restartCount > 0 && recentRestart:
		return "↑" + item.Restarts
	case anyRecentRestart:
		return " " + item.Restarts
	default:
		return item.Restarts
	}
}

// formatTableRowOrdered builds a plain-text table row using the given column
// order. "Name" is rendered at its position within order; an order without a
// "Name" entry renders no name cell (the column is hidden). The preprocessed
// values (ns, ready, restarts, status, age) are passed through since they have
// row-specific handling upstream (e.g. the cursor row preprocesses restarts for
// arrow alignment).
func formatTableRowOrdered(name, ns, ready, restarts, status, age string,
	nameW, contextW, nsW, readyW, restartsW, statusW, ageW int,
	order []string, extraCols []extraColumn, item *model.Item,
) string {
	widths := builtinColWidths{context: contextW, ns: nsW, ready: readyW, restarts: restartsW, status: statusW, age: ageW}
	inputs := plainCellInputs{item: item, ns: ns, ready: ready, restarts: restarts, status: status, age: age, widths: widths}
	var row strings.Builder
	for _, key := range order {
		if key == "Name" {
			row.WriteString(plainNameCellWithBadge(name, item, nameW))
			continue
		}
		if col := renderableBuiltin(key, widths); col != nil {
			row.WriteString(col.plain(inputs))
			continue
		}
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
// column order. The Name cell (with icon + badge handling) is rendered at its
// position within order via the styled name helper; an order without a "Name"
// entry renders no name cell (the column is hidden).
func formatTableRowStyledOrdered(item model.Item,
	nameW, contextW, nsW, readyW, restartsW, statusW, ageW int,
	order []string, extraCols []extraColumn, anyRecentRestart bool, nameOverride *lipgloss.Style,
) string {
	widths := builtinColWidths{context: contextW, ns: nsW, ready: readyW, restarts: restartsW, status: statusW, age: ageW}
	inputs := styledCellInputs{item: item, widths: widths, anyRecentRestart: anyRecentRestart}
	var base strings.Builder
	for _, key := range order {
		if key == "Name" {
			base.WriteString(styledNameCell(item, nameW, nameOverride))
			continue
		}
		if col := renderableBuiltin(key, widths); col != nil {
			base.WriteString(col.styled(inputs))
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

// plainNameCellWithBadge renders the name column for the plain-text path,
// appending the security severity badge inside the budget when one applies.
// Used for cursor rows where the highlighted background must not collide
// with ANSI styling embedded in the badge string.
func plainNameCellWithBadge(name string, item *model.Item, nameW int) string {
	badge := ""
	if item != nil {
		badge = securityBadgePlainForItem(item)
	}
	if badge == "" {
		return padRight(Truncate(name, nameW-1), nameW)
	}
	badgeW := lipgloss.Width(badge)
	reserved := badgeW + 2 // 1 separator + 1 column gap
	if reserved >= nameW {
		return padRight(Truncate(name, nameW-1), nameW)
	}
	nameMax := nameW - reserved
	trimmed := Truncate(name, nameMax)
	content := trimmed + " " + badge
	return padRight(content, nameW)
}

// styledNameCell renders the Name column with optional icon and dimmed
// styling for completed items. Pods in Succeeded or Completed status get
// their name dimmed; otherwise NormalStyle is used. The active highlight
// query is applied to the resolved display name.
//
// When a security finding index is active and the item has matching findings,
// the styled severity badge is appended inside the column budget (name is
// truncated to make room). Gated callers (ActiveSecurityAvailable == false)
// get an empty badge and the row renders identically to the pre-security UI.
func styledNameCell(item model.Item, nameW int, nameOverride *lipgloss.Style) string {
	// Ignored security findings (revealed by the show-ignored toggle, tagged
	// __ignored__ by groupFindings / GetSecurityAffectedResources) are dimmed
	// so they read as de-emphasized next to active findings.
	isDimmed := item.Status == "Succeeded" || item.Status == "Completed" ||
		item.ColumnValue("__ignored__") == "true"
	nameStyle := NormalStyle
	if isDimmed {
		nameStyle = DimStyle
	}
	// A row-status-tint override (name-only foreground tint, issue #540) recolors
	// the name text and wins over the default/dimmed style.
	if nameOverride != nil {
		nameStyle = *nameOverride
	}
	badge := securityBadgeForItem(&item)
	badgeW := lipgloss.Width(badge)
	badgeReserve := 0
	if badgeW > 0 {
		badgeReserve = badgeW + 1 // separator space
	}
	if resolvedIcon := iconCell(item.Icon); resolvedIcon != "" {
		iconSt := IconStyle
		if isDimmed {
			iconSt = DimStyle
		}
		icon := iconSt.Render(resolvedIcon) + " "
		iconVisualW := lipgloss.Width(icon)
		// -1 reserves gap before next column.
		nameRemaining := max(nameW-iconVisualW-1-badgeReserve, 1)
		// Drop the badge when it would not fit alongside a readable name.
		activeBadge := badge
		if badgeReserve > 0 && nameW-iconVisualW-1 <= badgeReserve {
			activeBadge = ""
			nameRemaining = max(nameW-iconVisualW-1, 1)
		}
		namePart := Truncate(item.Name, nameRemaining)
		if ActiveHighlightQuery != "" {
			namePart = highlightName(namePart, ActiveHighlightQuery)
		}
		nameVisualW := lipgloss.Width(namePart)
		badgeSegment := ""
		badgeSegmentW := 0
		if activeBadge != "" {
			badgeSegment = " " + activeBadge
			badgeSegmentW = lipgloss.Width(badgeSegment)
		}
		pad := max(nameW-iconVisualW-nameVisualW-badgeSegmentW, 0)
		if isDimmed || nameOverride != nil {
			namePart = nameStyle.Render(namePart)
		}
		return icon + namePart + badgeSegment + strings.Repeat(" ", pad)
	}
	// Drop the badge when it would not fit alongside a readable name.
	activeBadge := badge
	if badgeReserve > 0 && nameW <= badgeReserve+1 {
		activeBadge = ""
		badgeReserve = 0
	}
	nameRemaining := max(nameW-1-badgeReserve, 1)
	displayName := Truncate(item.Name, nameRemaining)
	if ActiveHighlightQuery != "" {
		displayName = highlightName(displayName, ActiveHighlightQuery)
	}
	if activeBadge == "" {
		return nameStyle.Render(padRight(displayName, nameW))
	}
	nameVisualW := lipgloss.Width(displayName)
	badgeSegment := " " + activeBadge
	badgeSegmentW := lipgloss.Width(badgeSegment)
	pad := max(nameW-nameVisualW-badgeSegmentW, 0)
	return nameStyle.Render(displayName) + badgeSegment + strings.Repeat(" ", pad)
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
	case "Severity":
		return severityColumnStyle(val)
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
		return extraColumnValueStyle(key, val)
	}
}

// extraColumnValueStyle colors low-cardinality printer/extra column values:
// boolean values follow the column name's condition polarity (Established/False
// is a problem, Failed/False is healthy), recognized status words reuse the
// shared status severity colors. Everything else stays dim.
func extraColumnValueStyle(key, val string) lipgloss.Style {
	if canonical, ok := canonicalBoolStatus(val); ok {
		// ALLCAPS headers would tokenize letter-by-letter in the polarity
		// heuristic; lowercase them so "ESTABLISHED" matches like "Established".
		if key == strings.ToUpper(key) {
			key = strings.ToLower(key)
		}
		return ConditionStyle(key, canonical)
	}
	switch statusSeverity(val) {
	case sevRunning, sevDone, sevProgressing, sevFailed:
		return StatusStyle(val)
	default:
		return DimStyle
	}
}

// canonicalBoolStatus normalizes boolean-ish column values to condition-status
// casing. CRD printer columns of type boolean render lowercase ("true"), while
// condition-backed JSONPath columns yield "True"/"False"/"Unknown".
func canonicalBoolStatus(val string) (string, bool) {
	switch {
	case strings.EqualFold(val, "true"):
		return "True", true
	case strings.EqualFold(val, "false"):
		return "False", true
	case strings.EqualFold(val, "unknown"):
		return "Unknown", true
	}
	return "", false
}

// severityColumnStyle returns the lipgloss.Style that paints the
// abbreviated severity label in the Severity column. Mirrors the badge
// palette in styleSeverityBadge so the row, badge, and details panel
// all use the same color per level.
func severityColumnStyle(val string) lipgloss.Style {
	switch val {
	case "CRIT":
		return StatusFailed
	case "HIGH":
		return DeprecationStyle
	case "MED":
		return StatusProgressing
	case "LOW":
		return StatusRunning
	}
	return DimStyle
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

// ParseResourceValue parses a CPU (millicores) or memory (bytes) string back to
// int64. Unparseable input (empty, "n/a", garbage) yields 0; callers that must
// distinguish "missing" from a genuine 0 should use ParseResourceValueOK.
func ParseResourceValue(val string, isCPU bool) int64 {
	v, _ := ParseResourceValueOK(val, isCPU)
	return v
}

// ParseResourceValueOK parses a CPU (millicores) or memory (bytes) string back
// to int64, reporting whether the value was a real numeric quantity. A leading
// trend arrow ("↑ " / "↓ ") is stripped first — it is a cosmetic decoration the
// metrics path prepends to the CPU/MEM columns, never part of the number. The
// ok flag is false for empty, "n/a", or otherwise unparseable input, letting
// sort comparators push metrics-less rows to the bottom instead of treating
// them as 0 (which is indistinguishable from a genuine "0m").
func ParseResourceValueOK(val string, isCPU bool) (int64, bool) {
	val = strings.TrimSpace(stripTrendArrow(val))
	if val == "" || val == "n/a" {
		return 0, false
	}
	if isCPU {
		// CPU: "100m" or "1.5" (cores)
		if before, ok := strings.CutSuffix(val, "m"); ok {
			n, err := strconv.ParseFloat(before, 64)
			if err != nil {
				return 0, false
			}
			return int64(n), true
		}
		n, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return 0, false
		}
		return int64(n * 1000), true
	}
	// Memory: "128Mi", "1.5Gi", "1024Ki", "1024B"
	var mult float64 = 1
	switch {
	case strings.HasSuffix(val, "Gi"):
		val, mult = strings.TrimSuffix(val, "Gi"), 1024*1024*1024
	case strings.HasSuffix(val, "Mi"):
		val, mult = strings.TrimSuffix(val, "Mi"), 1024*1024
	case strings.HasSuffix(val, "Ki"):
		val, mult = strings.TrimSuffix(val, "Ki"), 1024
	case strings.HasSuffix(val, "B"):
		val = strings.TrimSuffix(val, "B")
	}
	n, err := strconv.ParseFloat(val, 64)
	if err != nil {
		return 0, false
	}
	return int64(n * mult), true
}

// stripTrendArrow removes the leading "↑ " / "↓ " trend decoration the metrics
// refresh prepends to CPU/MEM usage values, returning the bare quantity.
func stripTrendArrow(val string) string {
	if rest, ok := strings.CutPrefix(val, "↑ "); ok {
		return rest
	}
	if rest, ok := strings.CutPrefix(val, "↓ "); ok {
		return rest
	}
	return val
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

// TruncateStart truncates a string from the left to maxW visual columns,
// prefixing "~" when it was cut. The mirror of Truncate, for text whose end
// matters more than its start: a text input's caret sits after the last
// character, so the tail is the part the user needs to see while typing.
func TruncateStart(s string, maxW int) string {
	if maxW <= 0 {
		return ""
	}
	w := lipgloss.Width(s)
	if w <= maxW {
		return s
	}
	if maxW <= 1 {
		return "~"
	}
	return ansi.TruncateLeft(s, w-maxW+1, "~")
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
