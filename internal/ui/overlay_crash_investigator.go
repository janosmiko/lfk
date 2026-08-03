package ui

import (
	"fmt"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
)

// CrashTab is the active tab in the crash investigator overlay.
type CrashTab int

const (
	// CrashTabSummary is the per-container summary view.
	CrashTabSummary CrashTab = iota
	// CrashTabEvents shows pod-scoped Kubernetes events.
	CrashTabEvents
	// CrashTabLogs shows logs for the active container (current or previous).
	CrashTabLogs
	// CrashTabDescribe shows the kubectl describe pod output.
	CrashTabDescribe
)

// CrashInvestigatorEntry is the presentation-only struct passed to the
// renderer. The `app` package builds it from `k8s.CrashInvestigation`
// before calling `RenderCrashInvestigatorOverlay`, so the `ui` package
// stays independent of `k8s`.
type CrashInvestigatorEntry struct {
	PodName         string
	Namespace       string
	Phase           string
	PodIP           string
	Node            string
	QoSClass        string
	Age             time.Duration
	OwnerKind       string
	OwnerName       string
	InitContainers  []CrashContainerEntry
	AppContainers   []CrashContainerEntry
	Events          []CrashEventEntry
	Describe        string
	DescribeError   string
	ActiveContainer string
	Tab             CrashTab
	ShowPrevious    bool
}

// CrashContainerEntry is a per-container row in the Summary tab and
// the source for the Logs/Describe tabs (selected via ActiveContainer).
type CrashContainerEntry struct {
	Name         string
	IsInit       bool
	Image        string
	State        string
	StateReason  string
	Ready        bool
	RestartCount int32
	LastReason   string
	LastExitCode int32
	LastSignal   int32
	LastFinished time.Time
	LastMessage  string
	HasLastTerm  bool
	PreviousLog  string
	CurrentLog   string
	LogError     string
}

// CrashEventEntry is one row in the Events tab.
type CrashEventEntry struct {
	Type    string
	Reason  string
	Age     string // pre-formatted "5m ago"
	Source  string
	Message string
}

// Column widths for the Summary container sub-table. Sum (with the
// 2-space gutter between cells) is well under the typical overlay width
// of min(110, m.width-6) so the table never wraps on a normal terminal.
const (
	crashColContainer  = 22
	crashColState      = 18
	crashColRestarts   = 9
	crashColLastExit   = 10
	crashColLastReason = 22
)

// crashTabSeparatorStyle styles the horizontal divider drawn between the
// tab bar and the body — replaces the previous nested rounded-border
// inner panel, which was visually redundant with the outer OverlayStyle
// frame and caused width-overflow soft-wraps that mangled borders. A
// single-line divider plus surface-bg fill gives clear visual hierarchy
// without doubling chrome columns.
var crashTabSeparatorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color(ColorBorder)).
	Background(SurfaceBg)

// crashSectionStyle styles Summary section headers ("Init Containers",
// "Containers", "Last termination of <name>") so they stand out from the
// body lines without using a separate bordered sub-panel per section.
// SurfaceBg is matched by ApplyTheme / applyNoColorTheme.
var crashSectionStyle = lipgloss.NewStyle().
	Bold(true).
	Underline(true).
	Foreground(lipgloss.Color(ColorPrimary)).
	Background(SurfaceBg)

// crashHeaderStyle styles the top-of-tab header on the Logs / Describe /
// Events tabs (e.g. "LOGS · previous · container=app"). Bold + primary
// color gives the header a clear visual weight above the body. SurfaceBg
// is matched by ApplyTheme / applyNoColorTheme.
var crashHeaderStyle = lipgloss.NewStyle().
	Bold(true).
	Foreground(lipgloss.Color(ColorPrimary)).
	Background(SurfaceBg)

// RenderCrashInvestigatorOverlay renders the full crash investigator
// overlay body (excluding the surrounding overlay frame, which the
// caller paints via OverlayStyle).
//
// Layout:
//
//	Title           (Crash Investigator)
//	Subtitle        (namespace/pod · container: <name>)
//	[blank line]
//	Tab bar         (Summary  Events  Logs  Describe — 2-space gutters)
//	Divider         (─────────────────────────────────…)
//	Body            (the active tab's clipped body)
//
// width/height are the overlay's outer dimensions as passed to
// OverlayStyle.Width/Height — that style reserves 2 cols of horizontal
// padding on EACH side of the rendered content, so the usable content
// area is `width - 4` cols. The renderer reserves 5 lines of chrome
// (title + subtitle + blank + tab bar + divider), then clips the body
// to the remaining viewport. scroll is clamped against the body length
// so 999999 (the G sentinel) lands on the last page.
//
// The previous redesign nested a second rounded-border panel around the
// body, mirroring the label/secret editor pattern. That doubled the
// horizontal chrome and pushed the visible row beyond the terminal width
// on tighter terminals; the terminal then soft-wrapped each row and the
// next row's `│` border appeared mid-line on the wrap continuation. The
// single-divider layout removes that nested chrome while keeping the
// visual hierarchy.
func RenderCrashInvestigatorOverlay(entry CrashInvestigatorEntry, scroll, width, height int) string {
	// OverlayStyle wraps our output with Padding(1, 2): 2 cols of padding
	// on each side of the content. The usable horizontal area for content
	// (and the divider) is therefore width - 4. Lines longer than that
	// get soft-wrapped by lipgloss into the next visible row, breaking
	// the visual layout (the user reported this as a wrapping divider).
	contentW := max(width-4, 20)
	// Chrome rows are: title (2 — OverlayTitleStyle adds Padding(0,0,1,0)
	// so the title renders as 2 visual rows), subtitle (1), blank (1),
	// tab bar (1), divider (1). Total = 6. Mismatching this against the
	// actual chrome height makes the body 1 row taller than the overlay
	// height and OverlayStyle.Height(H) extends the box to fit instead
	// of clipping — which is what the user saw as "the height grows".
	const chromeLines = 6
	bodyHeight := max(height-chromeLines, 1)

	// Clamp chrome rows so a long pod name in the subtitle can't overflow
	// and break alignment with the divider / body rows.
	title := clampLineWidth(OverlayTitleStyle.Render("Crash Investigator"), contentW)
	subtitle := clampLineWidth(OverlayDimStyle.Render(fmt.Sprintf(
		"%s/%s · container: %s",
		strings.TrimSpace(entry.Namespace),
		strings.TrimSpace(entry.PodName),
		fallbackCrashStr(entry.ActiveContainer),
	)), contentW)
	tabBar := clampLineWidth(renderCrashTabBar(entry.Tab), contentW)
	divider := crashTabSeparatorStyle.Render(strings.Repeat("─", contentW))

	body := renderCrashTabBody(entry, scroll, contentW, bodyHeight)
	// Defensive clamp: ensure every body line fits horizontally and the
	// total body line count never exceeds bodyHeight. lipgloss's bordered
	// container miscounts width when ANSI escapes are present (would
	// cause horizontal soft-wrap), and OverlayStyle.Height extends to fit
	// content longer than its declared height (would grow the box). Both
	// are user-visible bugs we've shipped already, so we enforce both
	// invariants here as a final layer.
	bodyLines := strings.Split(body, "\n")
	clampedLines := make([]string, 0, bodyHeight)
	for _, line := range bodyLines {
		if len(clampedLines) == bodyHeight {
			break
		}
		clampedLines = append(clampedLines, clampLineWidth(line, contentW))
	}
	for len(clampedLines) < bodyHeight {
		clampedLines = append(clampedLines, "")
	}
	body = strings.Join(clampedLines, "\n")

	return title + "\n" + subtitle + "\n\n" + tabBar + "\n" + divider + "\n" + body
}

// renderCrashTabBar renders the four-tab strip with the active tab
// highlighted via OverlaySelectedStyle (bg fill, not bold-only) and a
// 2-space gutter between tabs. Mirrors the label/secret editor's tab bar
// so the visual language is consistent across overlays.
func renderCrashTabBar(active CrashTab) string {
	tabs := []struct {
		label string
		t     CrashTab
	}{
		{" 1 Summary ", CrashTabSummary},
		{" 2 Events ", CrashTabEvents},
		{" 3 Logs ", CrashTabLogs},
		{" 4 Describe ", CrashTabDescribe},
	}
	parts := make([]string, 0, len(tabs))
	for _, t := range tabs {
		if t.t == active {
			parts = append(parts, OverlaySelectedStyle.Render(t.label))
		} else {
			parts = append(parts, OverlayDimStyle.Render(t.label))
		}
	}
	return strings.Join(parts, "  ")
}

func renderCrashTabBody(entry CrashInvestigatorEntry, scroll, width, height int) string {
	switch entry.Tab {
	case CrashTabSummary:
		return renderCrashSummaryTab(entry, scroll, width, height)
	case CrashTabEvents:
		return renderCrashEventsTab(entry, scroll, width, height)
	case CrashTabLogs:
		return renderCrashLogsTab(entry, scroll, width, height)
	case CrashTabDescribe:
		return renderCrashDescribeTab(entry, scroll, width, height)
	}
	return ""
}

// clipScrollLines slices lines to the visible window, clamping scroll
// against the maximum offset so a sentinel "jump to bottom" value (G,
// which writes 999999) lands on the last full page rather than past the
// end. Mirrors RenderAlertsOverlay's pattern.
func clipScrollLines(lines []string, scroll, height int) []string {
	if height <= 0 || len(lines) == 0 {
		return nil
	}
	maxScroll := max(len(lines)-height, 0)
	scroll = max(min(scroll, maxScroll), 0)
	end := min(scroll+height, len(lines))
	return lines[scroll:end]
}

// renderCrashSummaryTab renders the aggregated Summary tab body: pod-level
// header, init-container sub-table, app-container sub-table, and a
// last-termination detail block for the active container. Scroll is
// applied at line granularity so users can pan past long termination
// messages without losing the table header context.
func renderCrashSummaryTab(entry CrashInvestigatorEntry, scroll, _, height int) string {
	lines := buildCrashSummaryLines(entry)
	visible := clipScrollLines(lines, scroll, height)
	return strings.Join(visible, "\n")
}

// buildCrashSummaryLines flattens the Summary tab into a slice of
// pre-styled lines so scroll/clip can operate on a single dimension.
func buildCrashSummaryLines(entry CrashInvestigatorEntry) []string {
	lines := make([]string, 0, 16)

	// Pod-level header rendered as a vertical key/value block: each
	// metric on its own short line so a long Node name (or any future
	// added field) cannot exceed panelContentW and force the bordered
	// inner panel to soft-wrap. The previous single ` · `-separated
	// line broke the panel's right border on real-world pod metadata.
	labelStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorDimmed)).Background(SurfaceBg)
	valueStyle := OverlayNormalStyle

	kvLine := func(label, value string) string {
		if strings.TrimSpace(value) == "" {
			value = "—"
		}
		// Label width 8 chars (longest is "Owner: ").
		return "  " + labelStyle.Render(fmt.Sprintf("%-8s", label+":")) + "  " + valueStyle.Render(value)
	}

	lines = append(lines,
		kvLine("Phase", entry.Phase),
		kvLine("Node", entry.Node),
		kvLine("IP", entry.PodIP),
		kvLine("QoS", entry.QoSClass),
		kvLine("Age", formatDuration(entry.Age)),
	)
	if entry.OwnerKind != "" {
		lines = append(lines, kvLine("Owner", entry.OwnerKind+"/"+entry.OwnerName))
	}
	lines = append(lines, "")

	if len(entry.InitContainers) > 0 {
		lines = append(lines, "  "+crashSectionStyle.Render("Init Containers"))
		lines = append(lines, renderCrashContainerTableLines(entry.InitContainers, entry.ActiveContainer)...)
		lines = append(lines, "")
	}
	if len(entry.AppContainers) > 0 {
		lines = append(lines, "  "+crashSectionStyle.Render("Containers"))
		lines = append(lines, renderCrashContainerTableLines(entry.AppContainers, entry.ActiveContainer)...)
		lines = append(lines, "")
	}

	// Last-terminated detail block for the active container.
	if active := findCrashContainer(entry, entry.ActiveContainer); active != nil && active.HasLastTerm {
		lines = append(lines, "")
		lines = append(lines, "  "+crashSectionStyle.Render(fmt.Sprintf("Last termination of %s", active.Name)))
		lines = append(lines, OverlayDimStyle.Render(fmt.Sprintf(
			"    Reason: %s · ExitCode: %d · Signal: %d · Finished: %s",
			fallbackCrashStr(active.LastReason),
			active.LastExitCode,
			active.LastSignal,
			formatTimeAgo(active.LastFinished),
		)))
		if msg := strings.TrimSpace(active.LastMessage); msg != "" {
			lines = append(lines, OverlayDimStyle.Render("    Message: "+truncateCrash(msg, 200)))
		}
	}

	return lines
}

// renderCrashContainerTableLines renders the container sub-table as one
// pre-styled string per row (header + body). Cells are width-padded with
// lipgloss.Width so Unicode and themed bg/fg both align correctly. The
// active row is highlighted by applying OverlaySelectedStyle to the
// whole pre-padded row, matching the tab bar's active-tab visual
// language so the user sees the same "selected" treatment in both
// places.
func renderCrashContainerTableLines(containers []CrashContainerEntry, active string) []string {
	lines := make([]string, 0, len(containers)+1)

	// Header row uses unstyled cells joined by gutters, then the whole
	// string is rendered with OverlayDimStyle for a uniform dim bg.
	hdr := "  " + crashJoinRow(
		crashCell("CONTAINER", crashColContainer),
		crashCell("STATE", crashColState),
		crashCell("RESTARTS", crashColRestarts),
		crashCell("LAST EXIT", crashColLastExit),
		crashCell("LAST REASON", crashColLastReason),
	)
	lines = append(lines, OverlayDimStyle.Render(hdr))

	for _, c := range containers {
		marker := "  "
		if c.Name == active {
			marker = "→ "
		}
		state := c.State
		if c.StateReason != "" {
			state = c.StateReason
		}
		exit := "—"
		reason := "—"
		if c.HasLastTerm {
			exit = fmt.Sprintf("%d", c.LastExitCode)
			reason = fallbackCrashStr(c.LastReason)
		}
		row := marker + crashJoinRow(
			crashCell(truncateCrash(c.Name, crashColContainer), crashColContainer),
			crashCell(truncateCrash(state, crashColState), crashColState),
			crashCell(fmt.Sprintf("%d", c.RestartCount), crashColRestarts),
			crashCell(exit, crashColLastExit),
			crashCell(truncateCrash(reason, crashColLastReason), crashColLastReason),
		)
		if c.Name == active {
			lines = append(lines, OverlaySelectedStyle.Render(row))
		} else {
			lines = append(lines, OverlayNormalStyle.Render(row))
		}
	}
	return lines
}

// crashCell returns s padded (or truncated) to width using lipgloss.
// Using lipgloss.Width handles Unicode rune width correctly and is
// transparent to ANSI escapes; fmt.Sprintf("%-Ns", ...) miscounts both.
func crashCell(s string, width int) string {
	return lipgloss.NewStyle().Width(width).Render(s)
}

// crashJoinRow joins pre-padded cells with a 2-space gutter. Centralizes
// the gutter spacing so column widths stay in sync between header and body.
func crashJoinRow(cells ...string) string {
	return strings.Join(cells, "  ")
}

// findCrashContainer locates a container by name in either the init or
// app slice; returns nil if not found.
func findCrashContainer(entry CrashInvestigatorEntry, name string) *CrashContainerEntry {
	for i := range entry.InitContainers {
		if entry.InitContainers[i].Name == name {
			return &entry.InitContainers[i]
		}
	}
	for i := range entry.AppContainers {
		if entry.AppContainers[i].Name == name {
			return &entry.AppContainers[i]
		}
	}
	return nil
}

// truncateCrash returns s shortened to at most n runes (counting bytes for
// simplicity — container names and state strings are ASCII in practice).
// Adds an ellipsis when truncated.
func truncateCrash(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// wrapCrashText word-wraps s to at most width visual columns per line.
// Words longer than width are hard-broken at width boundaries so a single
// huge token (e.g. a container ID, JSON-encoded log line) cannot overflow
// the cell. Returns at least one line — empty input produces one empty
// line — so callers rendering inside table cells always get a
// deterministic row count.
//
// (The package already exposes a wrapText helper in explainview.go with
// different semantics: it returns nil on empty input and never hard-
// breaks. We need both behaviors here to keep table rows aligned, so
// this is a sibling helper rather than a replacement.)
func wrapCrashText(s string, width int) []string {
	if width <= 0 {
		return []string{""}
	}
	if s == "" {
		return []string{""}
	}
	var out []string
	var cur strings.Builder
	flush := func() {
		out = append(out, cur.String())
		cur.Reset()
	}
	for word := range strings.FieldsSeq(s) {
		// Hard-break tokens longer than the column.
		for lipgloss.Width(word) > width {
			room := width - cur.Len()
			if cur.Len() > 0 && room > 1 {
				cur.WriteByte(' ')
				room--
			} else if cur.Len() > 0 {
				flush()
				room = width
			}
			if room <= 0 {
				flush()
				room = width
			}
			cur.WriteString(word[:room])
			word = word[room:]
			flush()
		}
		next := word
		if cur.Len() > 0 {
			next = " " + word
		}
		if cur.Len()+lipgloss.Width(next) > width {
			flush()
			cur.WriteString(word)
		} else {
			cur.WriteString(next)
		}
	}
	if cur.Len() > 0 || len(out) == 0 {
		flush()
	}
	return out
}

// formatTimeAgo returns a human-readable "5m ago" string. The zero value
// renders as "—".
func formatTimeAgo(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return formatDuration(time.Since(t)) + " ago"
}

// renderCrashEventsTab renders the Events tab body. The header line
// declares the count and is followed by a thin divider so the user has a
// clear visual hierarchy above the column headers. Empty state shows a
// friendly "no events" message; otherwise a TYPE/REASON/AGE/MESSAGE
// table with Warning rows colored. Rows past the viewport are clipped
// using scroll; the renderer clamps so G (sentinel 999999) lands on
// the last page.
func renderCrashEventsTab(entry CrashInvestigatorEntry, scroll, width, height int) string {
	header := crashHeaderStyle.Render(fmt.Sprintf("EVENTS · %d", len(entry.Events)))
	divider := OverlayDimStyle.Render(strings.Repeat("─", min(width, 80)))

	if len(entry.Events) == 0 {
		out := header + "\n" + divider + "\n\n" + OverlayDimStyle.Render("  No events for this pod.")
		return out
	}

	// Reserve column space for the leading 4-space indent + fixed
	// columns + 2-space gutters; remainder is the message budget.
	const (
		typeW   = 7
		reasonW = 18
		ageW    = 7
		indent  = 4
		gutters = 6 // three 2-space gaps between four columns
	)
	msgW := max(width-indent-typeW-reasonW-ageW-gutters, 20)

	lines := make([]string, 0, len(entry.Events)*2+3)
	lines = append(lines, header, divider)
	hdr := strings.Repeat(" ", indent) + crashJoinRow(
		crashCell("TYPE", typeW),
		crashCell("REASON", reasonW),
		crashCell("AGE", ageW),
		"MESSAGE",
	)
	lines = append(lines, OverlayDimStyle.Render(hdr))

	// Continuation lines for wrapped messages start at the MESSAGE column,
	// so the visual cell stays aligned. msgPrefix is left-padding equal to
	// indent + typeW + reasonW + ageW + 3 gutters.
	msgPrefix := strings.Repeat(" ", indent+typeW+reasonW+ageW+gutters)

	for _, ev := range entry.Events {
		msgLines := wrapCrashText(ev.Message, msgW)
		first := strings.Repeat(" ", indent) + crashJoinRow(
			crashCell(truncateCrash(ev.Type, typeW), typeW),
			crashCell(truncateCrash(ev.Reason, reasonW), reasonW),
			crashCell(truncateCrash(ev.Age, ageW), ageW),
			msgLines[0],
		)
		style := OverlayNormalStyle
		if ev.Type == "Warning" {
			style = OverlayWarningStyle
		}
		lines = append(lines, style.Render(first))
		for _, cont := range msgLines[1:] {
			lines = append(lines, style.Render(msgPrefix+cont))
		}
	}
	visible := clipScrollLines(lines, scroll, height)
	return strings.Join(visible, "\n")
}

// renderCrashLogsTab renders the Logs tab body for the currently-active
// container. Header line declares the mode (previous|current) so the
// reader always knows which stream they're looking at, followed by a
// dim divider for visual hierarchy. The body is clipped to the viewport
// — long logs scroll with j/k/g/G/Ctrl+D/Ctrl+U.
//
// The header + divider are reserved as the top sticky lines; scroll
// only moves the body. That way users keep the "previous|current"
// context visible at all times.
func renderCrashLogsTab(entry CrashInvestigatorEntry, scroll, width, height int) string {
	active := findCrashContainer(entry, entry.ActiveContainer)
	mode := "current"
	if entry.ShowPrevious {
		mode = "previous"
	}
	header := crashHeaderStyle.Render(fmt.Sprintf("LOGS · %s · container=%s",
		mode, fallbackCrashStr(entry.ActiveContainer)))
	divider := OverlayDimStyle.Render(strings.Repeat("─", min(width, 80)))

	var b strings.Builder
	b.WriteString(header)
	b.WriteString("\n")
	b.WriteString(divider)
	b.WriteString("\n\n")

	// Reserve 3 lines for header + divider + blank; body has the rest.
	bodyHeight := max(height-3, 1)

	if active == nil {
		b.WriteString(OverlayDimStyle.Render("  No active container."))
		return b.String()
	}
	if active.LogError != "" {
		b.WriteString(OverlayWarningStyle.Render(fmt.Sprintf("  failed to load logs: %s", active.LogError)))
		return b.String()
	}

	body := active.CurrentLog
	if entry.ShowPrevious {
		body = active.PreviousLog
	}
	body = strings.TrimRight(body, "\n")
	if body == "" {
		if entry.ShowPrevious {
			b.WriteString(OverlayDimStyle.Render(
				"  no previous container output — this container has not been terminated yet. press p to view current logs."))
		} else {
			b.WriteString(OverlayDimStyle.Render("  no current logs available."))
		}
		return b.String()
	}

	// Wrap each raw log line to width-2 (account for the 2-col indent).
	// Long lines split into multiple visual rows instead of being
	// truncated, so users never lose information off the right edge.
	wrapW := max(width-2, 20)
	rawLines := strings.Split(body, "\n")
	styled := make([]string, 0, len(rawLines))
	for _, line := range rawLines {
		for _, chunk := range wrapCrashText(line, wrapW) {
			styled = append(styled, "  "+OverlayNormalStyle.Render(chunk))
		}
	}
	visible := clipScrollLines(styled, scroll, bodyHeight)
	b.WriteString(strings.Join(visible, "\n"))
	return b.String()
}

// renderCrashDescribeTab renders the Describe tab body — the kubectl
// describe output verbatim with a styled top header so the user has a
// clear orientation marker even when the body scrolls. If describe
// failed (e.g. kubectl not on PATH), we surface that as a warning
// instead. Long output is clipped to the viewport via scroll.
func renderCrashDescribeTab(entry CrashInvestigatorEntry, scroll, width, height int) string {
	header := crashHeaderStyle.Render("DESCRIBE · pod=" + fallbackCrashStr(entry.PodName))
	divider := OverlayDimStyle.Render(strings.Repeat("─", min(width, 80)))

	if entry.DescribeError != "" {
		return header + "\n" + divider + "\n\n" +
			OverlayWarningStyle.Render("  describe failed: "+entry.DescribeError)
	}
	body := strings.TrimRight(entry.Describe, "\n")
	if body == "" {
		return header + "\n" + divider + "\n\n" + OverlayDimStyle.Render("  no describe output.")
	}
	// Wrap each describe line to width-2 (account for the 2-col indent).
	// Long lines (e.g. "Annotations:" with a JSON-encoded value) split
	// onto multiple visual rows instead of being truncated.
	wrapW := max(width-2, 20)
	rawLines := strings.Split(body, "\n")
	styled := make([]string, 0, len(rawLines)+3)
	styled = append(styled, header, divider, "")
	for _, line := range rawLines {
		for _, chunk := range wrapCrashText(line, wrapW) {
			styled = append(styled, "  "+OverlayNormalStyle.Render(chunk))
		}
	}
	// Reserve the first 3 lines (header + divider + blank) as the sticky
	// top so scroll only moves the actual describe body underneath.
	stickyTop := styled[:3]
	bodyLines := styled[3:]
	bodyHeight := max(height-3, 1)
	visible := clipScrollLines(bodyLines, scroll, bodyHeight)
	return strings.Join(append(stickyTop, visible...), "\n")
}

// fallbackCrashStr returns an em-dash placeholder when s is blank
// (whitespace-only); otherwise returns s unchanged.
func fallbackCrashStr(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

// clampLineWidth truncates a line to at most maxWidth visible columns,
// using lipgloss's width-aware truncation so ANSI escapes are preserved
// but the display width never exceeds the panel content area. Without
// this, a single overlong content line forces lipgloss's bordered
// container to soft-wrap, which mangles the border characters.
func clampLineWidth(line string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if lipgloss.Width(line) <= maxWidth {
		return line
	}
	return lipgloss.NewStyle().MaxWidth(maxWidth).Render(line)
}
