package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// RenderErrorLogOverlay renders the application log overlay showing timestamped
// log entries with level indicators. The scroll parameter controls which portion is visible.
// When showDebug is false, DBG entries are filtered out.
func RenderErrorLogOverlay(entries []ErrorLogEntry, scroll int, height int, showDebug bool) string {
	var b strings.Builder
	b.WriteString(OverlayTitleStyle.Render("Application Log"))
	b.WriteString("\n")

	// Filter entries based on debug visibility.
	visible := make([]ErrorLogEntry, 0, len(entries))
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
			b.WriteString(CategoryStyle.Render("\u2500\u2500 " + it.label + " \u2500\u2500"))
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
			indicator = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Render("\u2713") // check mark
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
		b.WriteString(OverlayFilterStyle.Render("  " + q.Name))
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
	normalDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Render("\u25cf") // green filled circle
	warningDot := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError)).Render("\u25cf")    // red filled circle
	reasonStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorFile))
	sourceStyle := OverlayDimStyle
	countStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorWarning))

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
