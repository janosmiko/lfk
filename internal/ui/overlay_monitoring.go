package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// overlaySchemeScroll is the persistent scroll position for the colorscheme overlay.
var overlaySchemeScroll int

// ResetOverlaySchemeScroll resets the colorscheme overlay scroll to the top.
func ResetOverlaySchemeScroll() { overlaySchemeScroll = 0 }

// GetOverlaySchemeScroll returns the current colorscheme overlay scroll position.
func GetOverlaySchemeScroll() int { return overlaySchemeScroll }

// SchemeOverlayMaxVisible is the number of visible lines in the colorscheme overlay.
const SchemeOverlayMaxVisible = 20

// ErrorLogVisualParams holds visual selection state for the error log overlay.
type ErrorLogVisualParams struct {
	VisualMode     byte // 0 = off, 'v' = char, 'V' = line
	VisualStart    int  // anchor line index
	VisualStartCol int  // anchor column (for char mode)
	CursorLine     int  // current cursor line index
	CursorCol      int  // cursor column for char mode
}

// FilteredErrorLogEntries returns visible entries (respecting debug filter) in reverse chronological order.
func FilteredErrorLogEntries(entries []ErrorLogEntry, showDebug bool) []ErrorLogEntry {
	visible := make([]ErrorLogEntry, 0, len(entries))
	for _, e := range entries {
		if e.Level == "DBG" && !showDebug {
			continue
		}
		visible = append(visible, e)
	}
	reversed := make([]ErrorLogEntry, len(visible))
	for i, e := range visible {
		reversed[len(visible)-1-i] = e
	}
	return reversed
}

// ErrorLogEntryPlainText returns a plain text representation of a log entry for clipboard.
func ErrorLogEntryPlainText(e ErrorLogEntry) string {
	return fmt.Sprintf("%s [%s] %s", e.Time.Format("15:04:05"), e.Level, e.Message)
}

// errorLogLevelPalette returns the (foreground hex, bold) pair for a given
// log level. Unknown levels fall through to INF styling.
func errorLogLevelPalette(level string) (color, label string, bold bool) {
	switch level {
	case "ERR":
		return "#ff5555", "ERR", true
	case "WRN":
		return "#ffaa00", "WRN", true
	case "DBG":
		return "#6272a4", "DBG", false
	default:
		return "#888888", "INF", false
	}
}

// renderErrorLogLine formats a single error-log row: indicator + timestamp +
// styled level + message. When cursorBg is true, every segment inherits the
// cursor-line background so the level fg color stays visible while the row
// is highlighted. Non-cursor segments deliberately omit a background so the
// caller's FillLinesBg pass paints whichever bg fits the surrounding box
// (SurfaceBg in overlay mode, BaseBg in fullscreen).
func renderErrorLogLine(entry ErrorLogEntry, cursorBg bool) string {
	color, label, bold := errorLogLevelPalette(entry.Level)

	if cursorBg {
		base := lipgloss.NewStyle().Background(BaseBg).Bold(true)
		lvlStyle := lipgloss.NewStyle().Inherit(base).Foreground(ThemeColor(color))
		if bold {
			lvlStyle = lvlStyle.Bold(true)
		}
		ts := base.Render(entry.Time.Format("15:04:05"))
		lvl := lvlStyle.Render(label)
		msg := base.Render(entry.Message)
		sep := base.Render(" ")
		return base.Render(">") + sep + ts + sep + lvl + sep + msg
	}

	// Strip the surfaceBg the theme bakes into OverlayDimStyle and
	// OverlayNormalStyle so FillLinesBg can paint whichever bg fits
	// the surrounding box (SurfaceBg in the overlay form, BaseBg in
	// the fullscreen form). Otherwise the inner segments keep
	// SurfaceBg and clash with a BaseBg-filled fullscreen frame.
	ts := OverlayDimStyle.UnsetBackground().Render(entry.Time.Format("15:04:05"))
	lvlStyle := lipgloss.NewStyle().Foreground(ThemeColor(color))
	if bold {
		lvlStyle = lvlStyle.Bold(true)
	}
	lvl := lvlStyle.Render(label)
	return fmt.Sprintf("  %s %s %s", ts, lvl, OverlayNormalStyle.UnsetBackground().Render(entry.Message))
}

// RenderErrorLogOverlay renders the application log overlay showing timestamped
// log entries with level indicators. The scroll parameter controls which portion is visible.
// When showDebug is false, DBG entries are filtered out.
func RenderErrorLogOverlay(entries []ErrorLogEntry, scroll int, height int, showDebug bool, vp ErrorLogVisualParams) string {
	// Use bg-stripped variants of the overlay styles so the caller's
	// FillLinesBg pass paints whichever bg fits the surrounding box —
	// SurfaceBg for the bordered overlay form, BaseBg when this same
	// content is rendered as a fullscreen viewExplorer column.
	titleStyle := OverlayTitleStyle.UnsetBackground()
	dimStyle := OverlayDimStyle.UnsetBackground()

	var b strings.Builder
	b.WriteString(titleStyle.Render("Application Log"))
	b.WriteString("\n")

	reversed := FilteredErrorLogEntries(entries, showDebug)

	if len(reversed) == 0 {
		if len(entries) > 0 && !showDebug {
			b.WriteString(dimStyle.Render("No entries (debug logs hidden, press d to show)"))
		} else {
			b.WriteString(dimStyle.Render("No log entries"))
		}
		return b.String()
	}

	// Reserve lines for the title (1), blank line before footer (1), footer (1), and border padding.
	maxVisible := max(height-4, 1)

	// Clamp scroll.
	maxScroll := max(len(reversed)-maxVisible, 0)
	scroll = max(min(scroll, maxScroll), 0)

	end := min(scroll+maxVisible, len(reversed))

	// Visual selection range.
	selStart := min(vp.VisualStart, vp.CursorLine)
	selEnd := max(vp.VisualStart, vp.CursorLine)
	colStart := min(vp.VisualStartCol, vp.CursorCol)
	colEnd := max(vp.VisualStartCol, vp.CursorCol)

	for i := scroll; i < end; i++ {
		entry := reversed[i]
		plainText := ErrorLogEntryPlainText(entry)

		// Check if this line is in visual selection.
		inSelection := vp.VisualMode != 0 && i >= selStart && i <= selEnd
		isCursorLine := i == vp.CursorLine

		if inSelection {
			// Render with visual selection highlighting.
			rendered := RenderVisualSelection(
				plainText, rune(vp.VisualMode),
				i, selStart, selEnd,
				vp.VisualStart, vp.VisualStartCol, vp.CursorCol,
				colStart, colEnd,
			)
			b.WriteString("  " + rendered)
		} else if isCursorLine && vp.VisualMode == 0 {
			// Cursor line indicator (outside visual mode). Preserve the
			// level fg color by composing per-segment styles that inherit
			// the cursor-line bg, so red/orange ERR/WRN markers stay
			// visible when the user navigates through the overlay.
			b.WriteString(renderErrorLogLine(entry, true))
		} else {
			b.WriteString(renderErrorLogLine(entry, false))
		}
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	b.WriteString("\n\n")

	// Filter count for footer.
	visibleCount := len(reversed)
	scrollInfo := fmt.Sprintf("%d entries", visibleCount)
	if visibleCount != len(entries) {
		scrollInfo += fmt.Sprintf(" (%d hidden)", len(entries)-visibleCount)
	}
	if maxScroll > 0 {
		scrollInfo += fmt.Sprintf(" | scroll %d/%d", scroll+1, maxScroll+1)
	}
	if vp.VisualMode != 0 {
		modeLabel := "VISUAL LINE"
		if vp.VisualMode == 'v' {
			modeLabel = "VISUAL"
		}
		scrollInfo += " | " + modeLabel
	}
	b.WriteString(dimStyle.Render(scrollInfo))

	return b.String()
}

// RenderColorschemeOverlay renders the color scheme selector overlay content.
// entries is a list of SchemeEntry (with headers). cursor indexes only selectable entries.
func RenderColorschemeOverlay(entries []SchemeEntry, filter string, cursor int, filterMode bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Select Color Scheme"))
	b.WriteString("\n")

	// Filter input.
	switch {
	case filterMode:
		b.WriteString(OverlayFilterStyle.Render("/ " + filter + "\u2588"))
	case filter != "":
		b.WriteString(OverlayFilterStyle.Render("/ " + filter))
	default:
		b.WriteString(OverlayDimStyle.Render("/ to filter"))
	}
	b.WriteString("\n\n")

	// Build display list: when filtering, skip headers and filter selectable entries.
	type displayItem struct {
		label     string
		isHeader  bool
		selectIdx int // index among selectable items (-1 for headers)
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

	// Scrolling window with vim-style scrolloff for stable viewport.
	maxVisible := SchemeOverlayMaxVisible
	scrollOff := ConfigScrollOff
	if len(items) <= maxVisible {
		scrollOff = 0
	} else if maxSO := (maxVisible - 1) / 2; scrollOff > maxSO {
		scrollOff = maxSO
	}

	// Find the display index of the cursor item.
	cursorDisplayIdx := 0
	for i, it := range items {
		if !it.isHeader && it.selectIdx == cursor {
			cursorDisplayIdx = i
			break
		}
	}

	displayLines := func(from, to int) int { return to - from }
	start := VimScrollOff(overlaySchemeScroll, cursorDisplayIdx, len(items), maxVisible, scrollOff, displayLines)
	overlaySchemeScroll = start

	end := min(start+maxVisible, len(items))

	b.WriteString(RenderScrollAbove(start, end-start, len(items), 0))
	b.WriteString("\n")

	var lines []string
	for i := start; i < end; i++ {
		it := items[i]
		if it.isHeader {
			lines = append(lines, "") // separator line
			lines = append(lines, CategoryStyle.Render("\u2500\u2500 "+it.label+" \u2500\u2500"))
		} else {
			prefix := "  "
			if it.label == ActiveSchemeName {
				prefix = "* "
			}
			line := prefix + it.label
			if it.selectIdx == cursor {
				lines = append(lines, OverlaySelectedStyle.Render(line))
			} else {
				lines = append(lines, OverlayNormalStyle.Render(line))
			}
		}
	}

	// Pad or truncate to fixed height so the overlay doesn't resize.
	for len(lines) < maxVisible {
		lines = append(lines, "")
	}
	if len(lines) > maxVisible {
		lines = lines[:maxVisible]
	}
	b.WriteString(strings.Join(lines, "\n"))

	b.WriteString("\n")
	b.WriteString(RenderScrollBelow(start, end-start, len(items), 0))

	return b.String()
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

	return b.String()
}

// RenderRBACOverlay renders the RBAC permission check overlay content.
func RenderRBACOverlay(results []RBACCheckEntry, kind string) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render(fmt.Sprintf("RBAC Permissions: %s", kind)))
	b.WriteString("\n\n")

	for _, r := range results {
		indicator := OverlayWarningStyle.Render("\u2717") // cross mark
		if r.Allowed {
			indicator = lipgloss.NewStyle().Foreground(ThemeColor("2")).Background(SurfaceBg).Render("\u2713") // check mark
		}
		verb := OverlayNormalStyle.Render(fmt.Sprintf("  %-10s", r.Verb))
		b.WriteString(verb)
		b.WriteString(indicator)
		b.WriteString("\n")
	}

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
	fmt.Fprintf(&b, "  %s%s", OverlayInputStyle.Render(input), OverlayDimStyle.Render("\u2588"))
	return b.String()
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
	schedulingStyle := lipgloss.NewStyle().Foreground(ThemeColor("#7aa2f7")).Background(SurfaceBg) // blue
	pullStyle := lipgloss.NewStyle().Foreground(ThemeColor("#e0af68")).Background(SurfaceBg)       // yellow
	initStyle := lipgloss.NewStyle().Foreground(ThemeColor("#73daca")).Background(SurfaceBg)       // cyan
	containerStyle := lipgloss.NewStyle().Foreground(ThemeColor("#9ece6a")).Background(SurfaceBg)  // green
	readinessStyle := lipgloss.NewStyle().Foreground(ThemeColor("#bb9af7")).Background(SurfaceBg)  // purple
	inProgressStyle := lipgloss.NewStyle().Foreground(ThemeColor("#ff9e64")).Background(SurfaceBg) // orange
	unknownStyle := lipgloss.NewStyle().Foreground(ThemeColor("#565f89")).Background(SurfaceBg)    // dim

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
	barWidth := min(max(width-40, 10), 40)

	// Severity color styles.
	greenStyle := lipgloss.NewStyle().Foreground(ThemeColor("#9ece6a")).Background(SurfaceBg)
	yellowStyle := lipgloss.NewStyle().Foreground(ThemeColor("#e0af68")).Background(SurfaceBg)
	redStyle := lipgloss.NewStyle().Foreground(ThemeColor("#f7768e")).Background(SurfaceBg)

	for i, q := range quotas {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(OverlayFilterStyle.Render("  " + q.Name))
		b.WriteString("\n")

		for _, res := range q.Resources {
			// Resource name, left-aligned.
			nameLabel := fmt.Sprintf("    %-16s", res.Name)
			b.WriteString(OverlayNormalStyle.Render(nameLabel))

			// Build the usage bar.
			filled := max(min(int(res.Percent/100.0*float64(barWidth)), barWidth), 0)
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

	return b.String()
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
		return b.String()
	}

	// Reserve lines for header, blank line before footer, footer.
	maxLines := max(height-4, 1)

	// Content width inside OverlayStyle Padding(1,2) = 2 left + 2 right.
	contentWidth := width - 4

	// Calculate available width for message wrapping.
	msgIndent := "           "
	msgMaxWidth := max(contentWidth-len(msgIndent), 20)
	msgContIndent := msgIndent + "  "
	msgContWidth := max(msgMaxWidth-2, 10)

	// Calculate visual lines per event for scroll/viewport calculations.
	msgLineCount := func(idx int) int {
		msgLen := len([]rune(events[idx].Message))
		if msgLen <= msgMaxWidth {
			return 1
		}
		remaining := msgLen - msgMaxWidth
		return 1 + (remaining+msgContWidth-1)/msgContWidth
	}
	eventLines := func(idx int) int {
		return 1 + msgLineCount(idx) // 1 header line + message lines
	}

	// Clamp scroll: find max scroll where remaining events fill the viewport.
	if scroll < 0 {
		scroll = 0
	}
	if scroll >= len(events) {
		scroll = max(len(events)-1, 0)
	}
	// Shrink scroll if there's empty space at the bottom.
	for scroll > 0 {
		lines := 0
		for i := scroll; i < len(events); i++ {
			lines += eventLines(i)
		}
		if lines >= maxLines {
			break
		}
		scroll--
	}

	// Compute end index based on available visual lines.
	// Separators between events just terminate the previous line (already
	// counted in eventLines), they don't add extra visual lines.
	usedLines := 0
	end := scroll
	for end < len(events) {
		el := eventLines(end)
		if usedLines+el > maxLines {
			break
		}
		usedLines += el
		end++
	}
	if end == scroll && end < len(events) {
		usedLines += eventLines(end)
		end++
	}

	// Styles for event type indicators.
	normalDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Background(SurfaceBg).Render("\u25cf") // green filled circle
	warningDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Background(SurfaceBg).Render("\u25cf")    // red filled circle
	reasonStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFile)).Background(SurfaceBg)
	sourceStyle := OverlayDimStyle
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Background(SurfaceBg)

	for i := scroll; i < end; i++ {
		event := events[i]

		// Relative timestamp.
		ts := RelativeTime(event.Timestamp)
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

		// First line: timestamp, dot, reason, source, involved, count.
		line := fmt.Sprintf("  %s %s %s%s%s%s", tsStr, dot, reason, src, involved, countStr)
		b.WriteString(line)
		b.WriteString("\n")

		// Message lines: wrap long messages instead of truncating.
		// Continuation lines get extra indentation to distinguish them.
		msg := event.Message
		msgRunes := []rune(msg)
		firstChunkEnd := min(msgMaxWidth, len(msgRunes))
		fmt.Fprintf(&b, "%s%s", msgIndent, OverlayNormalStyle.Render(string(msgRunes[:firstChunkEnd])))
		for start := firstChunkEnd; start < len(msgRunes); start += msgContWidth {
			chunkEnd := min(start+msgContWidth, len(msgRunes))
			chunk := string(msgRunes[start:chunkEnd])
			b.WriteString("\n")
			fmt.Fprintf(&b, "%s%s", msgContIndent, OverlayDimStyle.Render(chunk))
		}

		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad to fixed height so the footer stays in place.
	for usedLines < maxLines {
		b.WriteString("\n")
		usedLines++
	}
	b.WriteString("\n")

	// Scroll info (hints moved to main status bar).
	scrollInfo := fmt.Sprintf("%d events", len(events))
	if scroll > 0 || end < len(events) {
		scrollInfo += fmt.Sprintf(" | showing %d-%d", scroll+1, end)
	}
	b.WriteString(OverlayDimStyle.Render(scrollInfo))

	return b.String()
}

// EventViewerParams holds state for the rich event viewer rendering.
type EventViewerParams struct {
	Lines        []string // flat text lines (one per event)
	ResourceName string
	Scroll       int
	Cursor       int
	CursorCol    int
	Width        int
	Height       int
	Wrap         bool
	Fullscreen   bool
	VisualMode   byte // 0=off, 'v'=char, 'V'=line, 'B'=block
	VisualStart  int
	VisualCol    int
	SearchQuery  string
	SearchActive bool
	SearchInput  string
}

// RenderEventViewer renders the event viewer with cursor, visual selection,
// search highlighting, and fullscreen support.
func RenderEventViewer(p EventViewerParams) string {
	var b strings.Builder

	// Title with mode indicators.
	title := "Event Timeline"
	if p.ResourceName != "" {
		title += " - " + p.ResourceName
	}
	var indicators []string
	if p.Fullscreen {
		indicators = append(indicators, "FULLSCREEN")
	}
	if p.Wrap {
		indicators = append(indicators, "WRAP")
	}
	if p.VisualMode != 0 {
		switch p.VisualMode {
		case 'v':
			indicators = append(indicators, "VISUAL")
		case 'V':
			indicators = append(indicators, "VISUAL LINE")
		case 'B':
			indicators = append(indicators, "VISUAL BLOCK")
		}
	}
	if p.SearchQuery != "" {
		indicators = append(indicators, "/"+p.SearchQuery)
	}
	if len(indicators) > 0 {
		title += " [" + strings.Join(indicators, " | ") + "]"
	}
	b.WriteString(OverlayTitleStyle.Render(title))
	b.WriteString("\n")

	if len(p.Lines) == 0 {
		b.WriteString(OverlayDimStyle.Render("No events found"))
		return b.String()
	}

	// Calculate visible area.
	maxVisible := max(p.Height-4, 1) // reserve for title, blank, footer, padding

	// Clamp scroll.
	maxScroll := max(len(p.Lines)-maxVisible, 0)
	scroll := max(min(p.Scroll, maxScroll), 0)

	end := min(scroll+maxVisible, len(p.Lines))

	// Visual selection range.
	selStart := min(p.VisualStart, p.Cursor)
	selEnd := max(p.VisualStart, p.Cursor)
	colStart := min(p.VisualCol, p.CursorCol)
	colEnd := max(p.VisualCol, p.CursorCol)

	// Search query for highlighting.
	lowerQuery := strings.ToLower(p.SearchQuery)

	// Available content width.
	// Overlay mode: OverlayStyle adds border(2) + padding(4) = 6, plus 1 for gutter.
	// Fullscreen mode: no border/padding, just gutter + margin.
	contentW := p.Width - 7
	if p.Fullscreen {
		contentW = p.Width - 2
	}
	if contentW < 10 {
		contentW = 10
	}

	wrapStyle := lipgloss.NewStyle().Width(contentW)

	evLineCtx := eventLineContext{
		wrapStyle:  wrapStyle,
		contentW:   contentW,
		lowerQuery: lowerQuery,
		selStart:   selStart,
		selEnd:     selEnd,
		colStart:   colStart,
		colEnd:     colEnd,
	}
	for i := scroll; i < end; i++ {
		b.WriteString(renderEventViewerLine(p, i, evLineCtx))
		if i < end-1 {
			b.WriteString("\n")
		}
	}

	// Pad to fixed height.
	rendered := end - scroll
	for rendered < maxVisible {
		b.WriteString("\n")
		rendered++
	}
	b.WriteString("\n")

	// Search input / footer.
	if p.SearchActive {
		b.WriteString(OverlayFilterStyle.Render("/ " + p.SearchInput + "\u2588"))
	} else {
		// Footer info.
		info := fmt.Sprintf("%d events", len(p.Lines))
		if scroll > 0 || end < len(p.Lines) {
			info += fmt.Sprintf(" | line %d/%d", p.Cursor+1, len(p.Lines))
		} else {
			info += fmt.Sprintf(" | line %d", p.Cursor+1)
		}
		if p.VisualMode != 0 {
			lineCount := selEnd - selStart + 1
			info += fmt.Sprintf(" | %d selected", lineCount)
		}
		b.WriteString(OverlayDimStyle.Render(info))
	}

	return b.String()
}

// eventLineContext holds shared state for rendering individual event viewer lines.
type eventLineContext struct {
	wrapStyle  lipgloss.Style
	contentW   int
	lowerQuery string
	selStart   int
	selEnd     int
	colStart   int
	colEnd     int
}

// renderEventViewerLine renders a single line in the event viewer.
func renderEventViewerLine(p EventViewerParams, i int, ctx eventLineContext) string {
	line := p.Lines[i]
	inSelection := p.VisualMode != 0 && i >= ctx.selStart && i <= ctx.selEnd
	isCursorLine := i == p.Cursor

	fitLine := line
	if p.Wrap {
		fitLine = ctx.wrapStyle.Render(line)
	} else if len([]rune(fitLine)) > ctx.contentW {
		fitLine = string([]rune(fitLine)[:ctx.contentW])
	}

	if inSelection {
		selLine := line
		if len([]rune(selLine)) > ctx.contentW {
			selLine = string([]rune(selLine)[:ctx.contentW])
		}
		rendered := RenderVisualSelection(
			selLine, rune(p.VisualMode),
			i, ctx.selStart, ctx.selEnd,
			p.VisualStart, p.VisualCol, p.CursorCol,
			ctx.colStart, ctx.colEnd,
		)
		if isCursorLine {
			return YamlCursorIndicatorStyle.Render("\u258e") + rendered
		}
		return " " + rendered
	}

	if isCursorLine {
		return renderEventCursorLine(p, line, fitLine, ctx)
	}

	return renderEventNormalLine(p, line, fitLine, ctx)
}

// renderEventCursorLine renders the cursor line with gutter indicator and block cursor.
func renderEventCursorLine(p EventViewerParams, line, fitLine string, ctx eventLineContext) string {
	gutter := YamlCursorIndicatorStyle.Render("\u258e")
	if p.Wrap {
		displayLine := fitLine
		if p.SearchQuery != "" {
			displayLine = ctx.wrapStyle.Render(highlightEventSearchLine(line, ctx.lowerQuery))
		}
		return gutter + displayLine
	}
	displayLine := fitLine
	if p.SearchQuery != "" {
		displayLine = highlightEventSearchLine(displayLine, ctx.lowerQuery)
	}
	return gutter + RenderCursorAtCol(displayLine, fitLine, p.CursorCol)
}

// renderEventNormalLine renders a non-cursor, non-selected line.
func renderEventNormalLine(p EventViewerParams, line, fitLine string, ctx eventLineContext) string {
	if p.Wrap {
		displayLine := fitLine
		if p.SearchQuery != "" {
			displayLine = ctx.wrapStyle.Render(highlightEventSearchLine(line, ctx.lowerQuery))
		}
		return " " + displayLine
	}
	displayLine := fitLine
	if p.SearchQuery != "" {
		displayLine = highlightEventSearchLine(displayLine, ctx.lowerQuery)
	} else {
		displayLine = OverlayNormalStyle.Render(displayLine)
	}
	return " " + displayLine
}

// highlightEventSearchLine highlights search matches in a single line using
// the overlay styles. The query should be pre-lowered for case-insensitive matching.
func highlightEventSearchLine(line, lowerQuery string) string {
	if lowerQuery == "" {
		return OverlayNormalStyle.Render(line)
	}
	lowerLine := strings.ToLower(line)
	matchStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSelectedFg)).
		Background(lipgloss.Color(ColorWarning)).
		Bold(true)

	var result strings.Builder
	pos := 0
	for pos < len(line) {
		idx := strings.Index(lowerLine[pos:], lowerQuery)
		if idx < 0 {
			result.WriteString(OverlayNormalStyle.Render(line[pos:]))
			break
		}
		if idx > 0 {
			result.WriteString(OverlayNormalStyle.Render(line[pos : pos+idx]))
		}
		matchEnd := pos + idx + len(lowerQuery)
		result.WriteString(matchStyle.Render(line[pos+idx : matchEnd]))
		pos = matchEnd
	}
	return result.String()
}

// RelativeTime returns a human-readable relative time string (e.g., "2m ago", "1h ago", "3d ago").
func RelativeTime(t time.Time) string {
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

// RenderAlertsOverlay renders the Prometheus alerts overlay content.
// scroll controls the visible portion; width and height limit the overlay size.
func RenderAlertsOverlay(alerts []AlertEntry, scroll, width, height int) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Monitoring Overview \u2014 Active Alerts"))
	b.WriteString("\n")

	if len(alerts) == 0 {
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("  No active alerts found"))
		b.WriteString("\n\n")
		b.WriteString(OverlayDimStyle.Render("  Prometheus was queried in well-known namespaces"))
		b.WriteString("\n")
		b.WriteString(OverlayDimStyle.Render("  (monitoring, prometheus, observability, kube-prometheus-stack)"))
		return b.String()
	}

	// Build lines for all alerts, then apply scroll window.
	lines := make([]string, 0, len(alerts)*4)
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
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(SurfaceBg)
		case "warning":
			severityIcon = "\u25cf"
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Bold(true).Background(SurfaceBg)
		default:
			severityIcon = "\u25cf"
			severityStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorPrimary)).Background(SurfaceBg)
		}

		stateLabel := alert.State
		var stateStyle lipgloss.Style
		if alert.State == "firing" {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Bold(true).Background(SurfaceBg)
		} else {
			stateStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning)).Background(SurfaceBg)
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
	b.WriteString(OverlayDimStyle.Render(info))

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
