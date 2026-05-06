package ui

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// SyncWavePane mirrors the app-side syncWavePane enum. Determines
// highlight tier in the renderer: active pane's cursor row gets the
// bright Selected style; inactive pane's cursor row gets ParentHighlightStyle.
type SyncWavePane int

const (
	SyncWavePaneSidebar SyncWavePane = iota
	SyncWavePaneBody
)

// SyncWaveBodyCursor mirrors the app-side syncWaveBodyCursor struct.
// WaveIdx == -1 / ResourceIdx == -1 indicates a placeholder row (collapsed
// or empty phase). ResourceIdx == -1 with a valid WaveIdx points at a
// wave header row.
type SyncWaveBodyCursor struct {
	WaveIdx     int
	ResourceIdx int
}

// SyncWaveTimelineEntry is the presentation-ready value passed to
// RenderSyncWaveTimeline. It mirrors k8s.SyncWaveTimeline but lives in
// internal/ui so the renderer has zero internal/k8s imports — keeps the
// view layer pure.
//
// BodyScroll is the body pane's first-visible row offset (in flattened-
// row units) — applied to the right-pane content only. The sidebar is
// fixed-position; only the body scrolls. SidebarCursor / BodyCursor /
// ActivePane drive cursor highlight (active pane's cursor gets the
// bright SelectedFg + Primary style; inactive pane's cursor gets
// ParentHighlightStyle, matching the main browser's parent-pane
// convention).
//
// SinglePane collapses the sidebar so narrow viewports (< 50 cols) get
// a body-only fallback; the renderer falls through to a single-pane
// path when this is true.
//
// Collapsed mirrors the app-side `collapsed` map keyed by phase name
// (`"<phase>"`) and wave label (`"<phase>/<waveLabel>"`). The renderer
// reads this through flattenBodyRows to drop hidden waves' resources
// without re-deriving the keys.
//
// LoadingFrame indexes into the spinner table when Loading is true.
// The renderer wraps with modulo so the caller can advance unbounded.
type SyncWaveTimelineEntry struct {
	AppName       string
	AppNamespace  string
	LivePhase     string
	Revision      string
	LastOperation *SyncWaveLastOperation
	Phases        []SyncWavePhaseEntry
	// Loading is true between the fast skeleton fetch and the slow wave-
	// annotation fan-out. The header surfaces this as a spinner glyph +
	// "Loading wave map…" so the operator knows wave numbers are still
	// arriving and the overlay isn't frozen.
	Loading      bool
	LoadingFrame int

	// New for two-pane layout. Wired into the renderer in Task 5;
	// populated from app state in Task 6.
	SidebarCursor int
	BodyCursor    SyncWaveBodyCursor
	BodyScroll    int
	ActivePane    SyncWavePane
	SinglePane    bool            // when true, sidebar is hidden
	Collapsed     map[string]bool // mirror of app-side collapsed; both phase and wave keys
}

// SyncWaveLastOperation summarizes the previous (or current) operation
// for the header line.
type SyncWaveLastOperation struct {
	Phase      string
	Message    string
	StartedAt  time.Time
	FinishedAt time.Time
	Revision   string
}

// SyncWavePhaseEntry mirrors k8s.SyncWavePhase + a Collapsed flag the app
// layer toggles. There is no per-phase Scroll — scrolling is global on
// SyncWaveTimelineEntry.BodyScroll so j/k can lift the viewport across
// phase boundaries, not just within one phase's body.
type SyncWavePhaseEntry struct {
	Name      string
	Collapsed bool
	Focused   bool
	Waves     []SyncWaveBucketEntry
}

// SyncWaveBucketEntry mirrors k8s.SyncWaveBucket. Wave == math.MinInt
// renders as "?".
type SyncWaveBucketEntry struct {
	Wave      int
	Resources []SyncWaveResourceEntry
}

// SyncWaveResourceEntry mirrors k8s.SyncWaveResource — duplicated rather
// than imported so the renderer has no internal/k8s dependency.
type SyncWaveResourceEntry struct {
	Group, Kind, Namespace, Name string
	SyncStatus, HealthStatus     string
	HookPhase                    string
	OpStatus                     string
	Message                      string
	IsHook                       bool
}

// SyncWaveUnknownWave is the sentinel for "wave not known". Same value
// as k8s.unknownWave (math.MinInt) so the app layer needs no translation.
const SyncWaveUnknownWave = math.MinInt

// SyncWaveGlyph picks the leading status glyph for a row based on the
// strongest signal available: opStatus first (this is what the operator
// just produced), then sync+health, then health alone, then a neutral dot.
func SyncWaveGlyph(opStatus, syncStatus, healthStatus string) string {
	switch opStatus {
	case "Succeeded", "Synced":
		return "✓"
	case "Running":
		return "⟳"
	case "Failed", "Error", "SyncFailed":
		return "✗"
	}
	switch healthStatus {
	case "Progressing":
		return "⟳"
	case "Degraded":
		return "✗"
	}
	if syncStatus == "Missing" {
		return "✗"
	}
	if syncStatus == "Synced" && healthStatus == "Healthy" {
		return "✓"
	}
	return "○"
}

// formatRelative produces a short humanized age string for the header.
func formatRelative(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours())/24)
	}
}

// RenderSyncWaveTimeline returns the rendered overlay string. width and
// height are the INNER dimensions (the caller subtracts OverlayStyle's
// border + padding chrome before calling). Every emitted line is clamped
// to `width` visual cells so a long resource label can't soft-wrap and
// shift the next row's `│` border into the middle of the line. When
// `height > 0` the output is exactly `height` lines: header rows
// (full-width) + viewport rows of `sidebar │ body` (clipped to fit) +
// empty padding rows when the content is shorter than the viewport.
// Padding to a fixed height is what stops the overlay's outer box from
// visibly shrinking as the user scrolls the body.
//
// Layout:
//   - height <= 0: unbounded fallback (header + full body, joined). Used
//     by tests that pass height=0 to skip clipping.
//   - SinglePane || width < 50: header + body using the full width. The
//     sidebar is dropped so narrow viewports degrade gracefully.
//   - Otherwise: header + viewport rows of `sidebar + │ + body` where
//     the sidebar is `sidebarWidth` cells wide and the body is
//     `width - sidebarWidth - 1` (the -1 reserves a column for the
//     separator).
func RenderSyncWaveTimeline(entry SyncWaveTimelineEntry, width, height int) string {
	headerLines := buildSyncWaveHeader(entry, width)
	if height <= 0 {
		// Unbounded: emit header + body joined as today; useful for
		// tests that pass height=0 to skip clipping.
		body := buildBody(entry, width, 1000)
		return strings.Join(append(headerLines, body...), "\n")
	}

	viewportRows := max(0, height-len(headerLines))

	if entry.SinglePane || width < 50 {
		// Single-pane fallback: header + body using full width.
		bodyLines := buildBody(entry, max(width, 1), viewportRows)
		out := make([]string, 0, height)
		for _, line := range headerLines {
			out = append(out, Truncate(line, width))
		}
		out = append(out, bodyLines...)
		for len(out) < height {
			out = append(out, "")
		}
		return strings.Join(out, "\n")
	}

	// Two-pane layout. -1 reserves a column for the separator glyph.
	bodyW := max(width-sidebarWidth-1, 10)
	sidebarLines := buildSidebar(entry, viewportRows)
	bodyLines := buildBody(entry, bodyW, viewportRows)

	separator := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBorder)).Render("│")
	out := make([]string, 0, height)
	for _, line := range headerLines {
		out = append(out, Truncate(line, width))
	}
	for i := range viewportRows {
		var s, b string
		if i < len(sidebarLines) {
			s = sidebarLines[i]
		} else {
			s = padOrTruncate("", sidebarWidth)
		}
		if i < len(bodyLines) {
			b = bodyLines[i]
		} else {
			b = padOrTruncate("", bodyW)
		}
		out = append(out, s+separator+b)
	}
	return strings.Join(out, "\n")
}

// syncWaveSpinnerFrames is the braille glyph cycle for the loading
// indicator. 10 frames at 100ms = 1s/cycle, the standard cadence for
// terminal spinners. Kept package-private since it only feeds the header.
var syncWaveSpinnerFrames = [...]string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// buildSyncWaveHeader emits the fixed header rows (title, optional
// last-op, optional live-phase, divider). Always 2..4 lines.
func buildSyncWaveHeader(entry SyncWaveTimelineEntry, width int) []string {
	titleStyle := lipgloss.NewStyle().Bold(true)
	lines := []string{titleStyle.Render("Sync Wave Timeline: " + entry.AppName)}
	if entry.LastOperation != nil {
		when := formatRelative(entry.LastOperation.FinishedAt)
		if entry.LastOperation.FinishedAt.IsZero() {
			when = formatRelative(entry.LastOperation.StartedAt)
		}
		line := fmt.Sprintf("Last Sync: %s · %s", entry.LastOperation.Phase, when)
		if entry.LastOperation.Revision != "" {
			line += " · revision: " + entry.LastOperation.Revision
		}
		lines = append(lines, line)
	}
	if entry.LivePhase == "Running" {
		lines = append(lines, "Live phase: Running")
	}
	if entry.Loading {
		// Surface the in-flight wave-annotation fan-out so the operator
		// knows wave numbers are still arriving — without this, the
		// skeleton paints all resources at "wave ?" and there's no signal
		// to the user that this is transient. The animated spinner makes
		// it obvious the overlay isn't frozen even when the fan-out
		// takes 10s+ on large Apps.
		idx := entry.LoadingFrame % len(syncWaveSpinnerFrames)
		if idx < 0 {
			idx += len(syncWaveSpinnerFrames)
		}
		lines = append(lines, syncWaveSpinnerFrames[idx]+" Loading wave map…")
	}
	lines = append(lines, strings.Repeat("─", max(0, width)))
	return lines
}

// composeRight builds the right-hand status column for a resource row.
//   - Hook rows: "<OpStatus>" (e.g., "Succeeded").
//   - Non-hook with both sync + health: "Synced/Healthy".
//   - Otherwise: whichever non-empty field is set.
func composeRight(r SyncWaveResourceEntry) string {
	if r.IsHook {
		if r.HookPhase != "" {
			return r.HookPhase
		}
		return r.OpStatus
	}
	switch {
	case r.SyncStatus != "" && r.HealthStatus != "":
		return r.SyncStatus + "/" + r.HealthStatus
	case r.SyncStatus != "":
		return r.SyncStatus
	case r.HealthStatus != "":
		return r.HealthStatus
	default:
		return ""
	}
}

// bodyRowKind enumerates the kinds of rows the body pane can render.
type bodyRowKind int

const (
	bodyRowKindWaveHeader bodyRowKind = iota
	bodyRowKindResource
	bodyRowKindPlaceholder // "<phase> collapsed — Enter to expand"
)

// bodyRow is one logical row in the flattened body view. The renderer
// uses it to apply cursor highlighting; the key handler uses it to walk
// the cursor through the visible rows skipping collapsed waves' resources.
type bodyRow struct {
	kind        bodyRowKind
	waveIdx     int // -1 for placeholder
	resourceIdx int // -1 for wave header / placeholder
}

// flattenBodyRows produces the visible row sequence for a phase given
// the collapse state. Phase-collapse and empty phases collapse to a
// single placeholder row. Wave-collapse hides the wave's resources but
// keeps the wave header.
func flattenBodyRows(phase SyncWavePhaseEntry, collapsed map[string]bool) []bodyRow {
	if collapsed[phase.Name] || len(phase.Waves) == 0 {
		return []bodyRow{{kind: bodyRowKindPlaceholder, waveIdx: -1, resourceIdx: -1}}
	}
	out := make([]bodyRow, 0)
	for wi, wave := range phase.Waves {
		out = append(out, bodyRow{kind: bodyRowKindWaveHeader, waveIdx: wi, resourceIdx: -1})
		waveLabel := "wave ?"
		if wave.Wave != SyncWaveUnknownWave {
			waveLabel = fmt.Sprintf("wave %d", wave.Wave)
		}
		if collapsed[phase.Name+"/"+waveLabel] {
			continue
		}
		for ri := range wave.Resources {
			out = append(out, bodyRow{kind: bodyRowKindResource, waveIdx: wi, resourceIdx: ri})
		}
	}
	return out
}

// sidebarWidth is the fixed width of the sidebar pane. Sized to fit the
// longest standard ArgoCD phase name (PostSyncFail = 12 chars) plus
// chrome (▾/▸ marker + space + "(N)" suffix). 22 keeps it tight without
// wrapping common phase names.
const sidebarWidth = 22

// buildSidebar returns sidebar lines, padded to viewportRows count.
// Each line is exactly sidebarWidth cells wide. Highlight tiers:
//   - Active pane (entry.ActivePane == SyncWavePaneSidebar):
//     row at entry.SidebarCursor gets bright SelectedFg/Primary style.
//   - Inactive pane: row at entry.SidebarCursor gets ParentHighlightStyle.
//   - Empty phases render with a dim foreground for visual mute.
func buildSidebar(entry SyncWaveTimelineEntry, viewportRows int) []string {
	out := make([]string, 0, viewportRows)
	for i, phase := range entry.Phases {
		empty := len(phase.Waves) == 0
		marker := "▾"
		count := fmt.Sprintf("(%d)", phaseResourceCount(phase))
		if empty {
			marker = "▸"
			count = "(none)"
		}
		if phase.Collapsed && !empty {
			marker = "▸"
		}
		raw := fmt.Sprintf(" %s %s %s", marker, phase.Name, count)
		raw = padOrTruncate(raw, sidebarWidth)
		styled := raw
		switch {
		case i == entry.SidebarCursor && entry.ActivePane == SyncWavePaneSidebar:
			styled = activeRowStyle().Render(raw)
		case i == entry.SidebarCursor:
			styled = ParentHighlightStyle.Render(raw)
		case empty:
			styled = dimRowStyle().Render(raw)
		}
		out = append(out, styled)
		if len(out) >= viewportRows {
			break
		}
	}
	for len(out) < viewportRows {
		out = append(out, padOrTruncate("", sidebarWidth))
	}
	return out
}

// phaseResourceCount counts the resources in a phase across all waves.
func phaseResourceCount(phase SyncWavePhaseEntry) int {
	n := 0
	for _, w := range phase.Waves {
		n += len(w.Resources)
	}
	return n
}

// activeRowStyle returns the bright cursor-row style used when the
// owning pane is active. Foreground/background match the main browser's
// active selection convention (SelectedFg on Primary).
func activeRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorSelectedFg)).
		Background(lipgloss.Color(ColorPrimary)).
		Bold(true)
}

// dimRowStyle returns the muted foreground used for empty phases.
func dimRowStyle() lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDimmed))
}

// padOrTruncate ensures s is exactly width visual cells. Truncates with
// "~" if too long, pads with spaces if too short. Rune-safe via
// lipgloss.Width.
func padOrTruncate(s string, width int) string {
	w := lipgloss.Width(s)
	if w == width {
		return s
	}
	if w > width {
		return Truncate(s, width)
	}
	return s + strings.Repeat(" ", width-w)
}

// buildBody returns body lines for the entry's selected phase, padded to
// viewportRows. Each line is exactly bodyWidth cells wide. Highlight
// tier is determined by entry.ActivePane and entry.BodyCursor.
//
// Body rows come from flattenBodyRows applied to the selected phase, then
// rendered in order. BodyScroll slices off rows above the viewport; rows
// below the viewport are dropped. Empty padding rows fill any remainder.
func buildBody(entry SyncWaveTimelineEntry, bodyWidth, viewportRows int) []string {
	if len(entry.Phases) == 0 || entry.SidebarCursor < 0 || entry.SidebarCursor >= len(entry.Phases) {
		return padOrTruncateRows(nil, bodyWidth, viewportRows)
	}
	phase := entry.Phases[entry.SidebarCursor]
	rows := flattenBodyRows(phase, entry.Collapsed)

	rendered := make([]string, len(rows))
	for i, row := range rows {
		raw := renderBodyRow(phase, row, entry.Collapsed, bodyWidth)
		styled := raw
		isCursor := row.waveIdx == entry.BodyCursor.WaveIdx && row.resourceIdx == entry.BodyCursor.ResourceIdx
		if row.kind == bodyRowKindPlaceholder && entry.BodyCursor.WaveIdx == -1 && entry.BodyCursor.ResourceIdx == -1 {
			isCursor = true
		}
		switch {
		case isCursor && entry.ActivePane == SyncWavePaneBody:
			styled = activeRowStyle().Render(raw)
		case isCursor:
			styled = ParentHighlightStyle.Render(raw)
		}
		rendered[i] = styled
	}

	// Apply scroll.
	if entry.BodyScroll > 0 && entry.BodyScroll < len(rendered) {
		rendered = rendered[entry.BodyScroll:]
	} else if entry.BodyScroll >= len(rendered) {
		rendered = nil
	}

	// Clip to viewport.
	if len(rendered) > viewportRows {
		rendered = rendered[:viewportRows]
	}

	return padOrTruncateRows(rendered, bodyWidth, viewportRows)
}

// renderBodyRow emits the raw (unstyled) text for one body row. The
// caller applies cursor / inactive highlight after.
func renderBodyRow(phase SyncWavePhaseEntry, row bodyRow, collapsed map[string]bool, width int) string {
	switch row.kind {
	case bodyRowKindPlaceholder:
		text := fmt.Sprintf(" %s collapsed — Enter to expand", phase.Name)
		return padOrTruncate(text, width)
	case bodyRowKindWaveHeader:
		wave := phase.Waves[row.waveIdx]
		waveLabel := "wave ?"
		if wave.Wave != SyncWaveUnknownWave {
			waveLabel = fmt.Sprintf("wave %d", wave.Wave)
		}
		key := phase.Name + "/" + waveLabel
		marker := "▾"
		suffix := ""
		if collapsed[key] {
			marker = "▸"
			suffix = fmt.Sprintf(" (%d items)", len(wave.Resources))
		}
		text := fmt.Sprintf("  %s %s%s", marker, waveLabel, suffix)
		return padOrTruncate(text, width)
	case bodyRowKindResource:
		wave := phase.Waves[row.waveIdx]
		r := wave.Resources[row.resourceIdx]
		glyph := SyncWaveGlyph(r.OpStatus, r.SyncStatus, r.HealthStatus)
		label := r.Kind + "/" + r.Name
		right := composeRight(r)
		text := fmt.Sprintf("       %s %-40s %s", glyph, Truncate(label, 40), right)
		return padOrTruncate(text, width)
	}
	return padOrTruncate("", width)
}

// padOrTruncateRows ensures the output is exactly viewportRows long,
// each line exactly bodyWidth wide.
func padOrTruncateRows(rows []string, bodyWidth, viewportRows int) []string {
	out := make([]string, 0, viewportRows)
	for i := range viewportRows {
		if i < len(rows) {
			out = append(out, padOrTruncate(rows[i], bodyWidth))
			continue
		}
		out = append(out, padOrTruncate("", bodyWidth))
	}
	return out
}
