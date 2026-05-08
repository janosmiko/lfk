package ui

import (
	"strconv"
	"strings"
)

// CaptureOverlayEntry is the presentation-only struct passed to the renderer.
// The app layer translates k8s.CaptureEntry / captureOverlayState into this.
type CaptureOverlayEntry struct {
	// Header
	Title       string // "Traffic Capture — namespace/pod-xyz"
	HeaderHints string // "iface=any · backend=kubectl-debug"
	FilterText  string // "filter: not arp …"
	StatusBadge string // "● capturing 12s" / "■ stopped" / ""

	// Phase
	Phase CaptureOverlayPhase

	// Phase A focus model — which configuration field has keyboard focus.
	// Only meaningful in CapturePhaseConfig.
	ConfigFocus CaptureConfigFocus

	// Phase A: backend chips
	Backends []CaptureBackendChip
	// Phase A: iface picker
	Interface string
	// Phase A: snaplen
	SnapLen int
	// Phase A: filter input + presets
	FilterInputValue   string
	FilterInputCursor  int
	FilterPresets      []string
	FilterPresetCursor int

	// Phase A endpoint pick (Service)
	EndpointRows   []CaptureEndpointRow
	EndpointCursor int

	// Phase B/C: status panel
	PacketCount int64
	ByteCount   int64
	OutputPath  string

	// Phase B/C: live table
	ShowStatusOnly bool
	Packets        []PacketRow
	ScrollOffset   int

	// Last error from the underlying capture process. Shown in
	// CapturePhaseStopped when non-empty.
	LastError string
}

// CaptureOverlayPhase mirrors capturePhase in the app package.
type CaptureOverlayPhase int

const (
	CapturePhaseConfig CaptureOverlayPhase = iota
	CapturePhaseEndpointPick
	CapturePhaseLive
	CapturePhaseStopped
)

// CaptureConfigFocus identifies which Phase A field has keyboard focus.
// Default is CaptureFocusFilter so every printable keypress goes to the
// filter input — the most common path.
type CaptureConfigFocus int

const (
	CaptureFocusFilter CaptureConfigFocus = iota
	CaptureFocusBackend
	CaptureFocusInterface
	CaptureFocusPreset
)

// CaptureBackendChip is one row in the backend picker.
type CaptureBackendChip struct {
	Label     string
	Selected  bool
	Available bool
	Reason    string // shown when !Available
}

// CaptureEndpointRow is one row in the Service endpoint picker.
type CaptureEndpointRow struct {
	PodName string
	IP      string
	Ready   bool
}

// PacketRow is one row in the live packet table.
type PacketRow struct {
	Time     string
	Protocol string
	Src      string
	Dst      string
	Length   int
	Flags    string
	Extra    string
}

// RenderTrafficCaptureOverlay renders the inner content of the traffic
// capture overlay. The caller wraps the result with OverlayStyle, which
// supplies the rounded border and Padding(1, 2). contentW is the usable
// width inside that padding (caller computes it as overlayW - 4); contentH
// is the usable height (caller computes it as overlayH - 4).
//
// contentH bounds the live packet table — without it the rendered string
// grows unboundedly with the live packet count and visibly enlarges the
// overlay box past the configured height.
//
// This renderer does NOT add its own border or padding — that would
// double the chrome and cause long rows to soft-wrap.
func RenderTrafficCaptureOverlay(e CaptureOverlayEntry, contentW, contentH int) string {
	if contentW < 20 {
		contentW = 20
	}
	if contentH < 5 {
		contentH = 5
	}
	switch e.Phase {
	case CapturePhaseEndpointPick:
		return buildCaptureEndpointPick(e, contentW)
	case CapturePhaseLive, CapturePhaseStopped:
		return buildCaptureLive(e, contentW, contentH)
	default: // CapturePhaseConfig
		return buildCaptureConfig(e, contentW)
	}
}

// captureFocusMarker returns "▶ " for the focused row and "  " for others
// so every config row stays visually aligned.
func captureFocusMarker(active, current CaptureConfigFocus) string {
	if active == current {
		return "▶ "
	}
	return "  "
}

func buildCaptureConfig(e CaptureOverlayEntry, contentW int) string {
	var sb strings.Builder

	sb.WriteString(OverlayTitleStyle.Render(clampLineWidth(e.Title, contentW)))
	sb.WriteString("\n")
	sb.WriteString(OverlayDimStyle.Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n")

	// Backend row
	sb.WriteString(focusedLabel(e.ConfigFocus, CaptureFocusBackend, "Backend: "))
	for i, b := range e.Backends {
		var chip string
		switch {
		case b.Selected:
			// The currently-selected backend stays bold regardless of focus
			// so the user always sees what's active.
			chip = OverlaySelectedStyle.Render(" " + b.Label + " ")
		case !b.Available:
			label := b.Label
			if b.Reason != "" {
				label = b.Label + " (" + truncate(b.Reason, 30) + ")"
			}
			chip = OverlayDimStyle.Render(label)
		default:
			chip = OverlayNormalStyle.Render(b.Label)
		}
		sb.WriteString(chip)
		if i < len(e.Backends)-1 {
			sb.WriteString(OverlayNormalStyle.Render("  "))
		}
	}
	sb.WriteString("\n")

	// Iface row
	sb.WriteString(focusedLabel(e.ConfigFocus, CaptureFocusInterface, "Iface:   "))
	if e.ConfigFocus == CaptureFocusInterface {
		sb.WriteString(OverlaySelectedStyle.Render(" " + e.Interface + " "))
	} else {
		sb.WriteString(OverlayNormalStyle.Render(e.Interface))
	}
	sb.WriteString("\n")

	// Snaplen row (read-only for now)
	sb.WriteString(OverlayNormalStyle.Render(clampLineWidth("  Snaplen: "+strconv.Itoa(e.SnapLen), contentW)))
	sb.WriteString("\n")

	// Filter row — cursor only visible when focused.
	cursor := ""
	if e.ConfigFocus == CaptureFocusFilter {
		cursor = "█"
	}
	sb.WriteString(focusedLabel(e.ConfigFocus, CaptureFocusFilter, "Filter:  "))
	sb.WriteString(OverlayNormalStyle.Render(clampLineWidth(e.FilterInputValue+cursor, contentW-9)))
	sb.WriteString("\n")

	// Presets row.
	sb.WriteString(focusedLabel(e.ConfigFocus, CaptureFocusPreset, "Presets: "))
	for i, p := range e.FilterPresets {
		switch {
		case i == e.FilterPresetCursor && e.ConfigFocus == CaptureFocusPreset:
			sb.WriteString(OverlaySelectedStyle.Render(" " + p + " "))
		case i == e.FilterPresetCursor:
			sb.WriteString(OverlayNormalStyle.Bold(true).Render(p))
		default:
			sb.WriteString(OverlayDimStyle.Render(p))
		}
		if i < len(e.FilterPresets)-1 {
			sb.WriteString(OverlayDimStyle.Render(" · "))
		}
	}
	sb.WriteString("\n")
	// Hint bar lives in the bottom-of-screen status bar (overlayHintBarMisc).
	// Adding inline hints here would duplicate that surface; see overlay.go
	// for the project-wide convention.
	return sb.String()
}

// focusedLabel renders a row label with a focus marker when the row is
// active, falling back to plain dim style when it isn't.
func focusedLabel(active, current CaptureConfigFocus, label string) string {
	marker := captureFocusMarker(active, current)
	if active == current {
		return OverlayNormalStyle.Bold(true).Render(marker + label)
	}
	return OverlayDimStyle.Render(marker + label)
}

func buildCaptureEndpointPick(e CaptureOverlayEntry, contentW int) string {
	var sb strings.Builder
	sb.WriteString(OverlayTitleStyle.Render(clampLineWidth(e.Title, contentW)))
	sb.WriteString("\n")
	sb.WriteString(OverlayDimStyle.Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n")
	for i, r := range e.EndpointRows {
		marker := "  "
		if i == e.EndpointCursor {
			marker = "> "
		}
		readyTag := "ready"
		if !r.Ready {
			readyTag = "not-ready"
		}
		row := marker + r.PodName + "  " + r.IP + "  " + readyTag
		switch {
		case i == e.EndpointCursor && r.Ready:
			sb.WriteString(OverlaySelectedStyle.Render(clampLineWidth(row, contentW)))
		case r.Ready:
			sb.WriteString(OverlayNormalStyle.Render(clampLineWidth(row, contentW)))
		default:
			sb.WriteString(OverlayDimStyle.Render(clampLineWidth(row, contentW)))
		}
		sb.WriteString("\n")
	}
	if len(e.EndpointRows) == 0 {
		sb.WriteString(OverlayDimStyle.Render(clampLineWidth("  No endpoints for this service — nothing to capture.", contentW)))
		sb.WriteString("\n")
	}
	// Hint bar lives in the bottom-of-screen status bar (overlayHintBarMisc).
	return sb.String()
}

func buildCaptureLive(e CaptureOverlayEntry, contentW, contentH int) string {
	var sb strings.Builder
	header := e.Title
	if e.HeaderHints != "" {
		header += "  " + e.HeaderHints
	}
	sb.WriteString(OverlayTitleStyle.Render(clampLineWidth(header, contentW)))
	sb.WriteString("\n")

	filterLine := e.FilterText
	if e.StatusBadge != "" {
		if filterLine != "" {
			filterLine += "  " + e.StatusBadge
		} else {
			filterLine = e.StatusBadge
		}
	}
	if filterLine != "" {
		sb.WriteString(OverlayNormalStyle.Render(clampLineWidth(filterLine, contentW)))
		sb.WriteString("\n")
	}
	sb.WriteString(OverlayDimStyle.Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n")

	statusLine := "packets: " + strconv.FormatInt(e.PacketCount, 10) +
		" · bytes: " + formatCaptureBytes(e.ByteCount)
	if e.OutputPath != "" {
		statusLine += " · file: " + e.OutputPath
	}
	sb.WriteString(OverlayNormalStyle.Render(clampLineWidth(statusLine, contentW)))
	sb.WriteString("\n")

	// Surface LastError just under the status panel when present.
	if e.LastError != "" {
		sb.WriteString(OverlayDimStyle.Render(strings.Repeat("─", contentW)))
		sb.WriteString("\n")
		for line := range strings.SplitSeq(e.LastError, "\n") {
			sb.WriteString(OverlayNormalStyle.Render(clampLineWidth("error: "+line, contentW)))
			sb.WriteString("\n")
		}
	}

	sb.WriteString(OverlayDimStyle.Render(strings.Repeat("─", contentW)))
	sb.WriteString("\n")

	if !e.ShowStatusOnly {
		hdr := "TIME           PROTO  SRC                  DST                  LEN FLAGS"
		sb.WriteString(OverlayNormalStyle.Bold(true).Render(clampLineWidth(hdr, contentW)))
		sb.WriteString("\n")
		// Bound the visible table to the rows that fit in the remaining
		// overlay height so the box doesn't grow with packet count. Window
		// into the buffer using ScrollOffset; G (jump-to-bottom) sends a
		// huge ScrollOffset which we clamp to the last page.
		visibleRows := remainingRowsForPacketTable(sb.String(), contentH)
		start, end := windowPackets(len(e.Packets), e.ScrollOffset, visibleRows)
		for _, p := range e.Packets[start:end] {
			sb.WriteString(OverlayNormalStyle.Render(clampLineWidth(formatPacketRow(p), contentW)))
			sb.WriteString("\n")
		}
	}
	// Hint bar lives in the bottom-of-screen status bar (overlayHintBarMisc).
	return sb.String()
}

// remainingRowsForPacketTable returns how many table rows fit in the overlay
// after the chrome already written into `rendered`. Bounded to >= 1 so we
// always show at least the most recent packet on small terminals.
func remainingRowsForPacketTable(rendered string, contentH int) int {
	used := strings.Count(rendered, "\n")
	avail := contentH - used
	if avail < 1 {
		return 1
	}
	return avail
}

// windowPackets returns the [start, end) slice of a length-N packet buffer
// to render. scrollOffset is "rows back from the latest packet" — 0 means
// "show the most recent visible window" (tail -f), positive values reveal
// older history. This matches what users expect from a live packet table:
// new arrivals push old ones up; you scroll k/up to see history; G returns
// to live; g jumps to the oldest packet. Clamps so a huge scrollOffset
// (g key) lands on the oldest page rather than running past the buffer.
func windowPackets(n, scrollOffset, visible int) (int, int) {
	if n == 0 || visible <= 0 {
		return 0, 0
	}
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	maxOffset := max(n-visible, 0)
	if scrollOffset > maxOffset {
		scrollOffset = maxOffset
	}
	end := n - scrollOffset
	start := max(end-visible, 0)
	return start, end
}

func formatPacketRow(p PacketRow) string {
	row := p.Time + "  " + p.Protocol + "  " + p.Src + "  " + p.Dst + "  " +
		strconv.Itoa(p.Length) + "  " + p.Flags
	if p.Extra != "" {
		row += "  " + p.Extra
	}
	return row
}

func formatCaptureBytes(n int64) string {
	const (
		kib = int64(1024)
		mib = 1024 * kib
		gib = 1024 * mib
	)
	switch {
	case n >= gib:
		return formatCaptureFloat1(float64(n)/float64(gib)) + " GiB"
	case n >= mib:
		return formatCaptureFloat1(float64(n)/float64(mib)) + " MiB"
	case n >= kib:
		return formatCaptureFloat1(float64(n)/float64(kib)) + " KiB"
	default:
		return strconv.FormatInt(n, 10) + " B"
	}
}

func formatCaptureFloat1(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}

// truncate returns s if it fits in n runes, otherwise s truncated and "…".
func truncate(s string, n int) string {
	if n <= 0 {
		return ""
	}
	rs := []rune(s)
	if len(rs) <= n {
		return s
	}
	if n == 1 {
		return "…"
	}
	return string(rs[:n-1]) + "…"
}
