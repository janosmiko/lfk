package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// usageBarLabelWidth is the display width of the "CPU: " / "Mem: " row labels.
// The bar is sized to the section width minus this so the rendered line fills
// the full footer width.
const usageBarLabelWidth = 5

// usageBarLoadingText is shown in place of a bar while metrics load.
const usageBarLoadingText = "loading…"

// usageBarSuffixReserve is the fixed column width reserved for the value text
// after each bar. Fixing it keeps the bar a constant width across resources
// (no jumping as value strings vary). The value text hugs the bar (left-aligned)
// so a short value leaves trailing space, not a gap before the text.
// RenderResourceUsage widens the reservation only when a value is longer than
// this (rare), which shrinks the bar for that resource rather than overflowing.
const usageBarSuffixReserve = 20

// RenderResourceUsage renders CPU and memory usage bars for the preview.
// If request/limit values are zero, usage is shown without a percentage bar.
func RenderResourceUsage(cpuUsed, cpuReq, cpuLim, memUsed, memReq, memLim int64, width int) string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(BaseBg)

	// Reserve a fixed suffix column (widened only for an unusually long value)
	// so the bar stays a constant width across resources and both bars match.
	// Text-only rows (no request and no limit) carry no bar, so they don't
	// constrain the reserve.
	suffixWidth := usageBarSuffixReserve
	if cpuLim > 0 || cpuReq > 0 {
		suffixWidth = max(suffixWidth, lipgloss.Width(usageSuffixText(cpuUsed, cpuReq, cpuLim, FormatCPU)))
	}
	if memLim > 0 || memReq > 0 {
		suffixWidth = max(suffixWidth, lipgloss.Width(usageSuffixText(memUsed, memReq, memLim, FormatMemory)))
	}

	lines := make([]string, 0, 3)
	lines = append(lines, DimStyle.Bold(true).Render("RESOURCE USAGE"))
	lines = append(lines, labelStyle.Render("CPU: ")+renderUsageBar(cpuUsed, cpuReq, cpuLim, width-usageBarLabelWidth, FormatCPU, suffixWidth))
	lines = append(lines, labelStyle.Render("Mem: ")+renderUsageBar(memUsed, memReq, memLim, width-usageBarLabelWidth, FormatMemory, suffixWidth))

	return strings.Join(lines, "\n")
}

// RenderResourceUsagePlaceholder renders the RESOURCE USAGE section with a
// "loading" marker in place of the bars. Shown while real metrics load for a
// freshly focused resource so the footer never displays the previous
// resource's numbers.
func RenderResourceUsagePlaceholder() string {
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSecondary)).Bold(true).Background(BaseBg)

	lines := make([]string, 0, 3)
	lines = append(lines, DimStyle.Bold(true).Render("RESOURCE USAGE"))
	lines = append(lines, labelStyle.Render("CPU: ")+DimStyle.Render(usageBarLoadingText))
	lines = append(lines, labelStyle.Render("Mem: ")+DimStyle.Render(usageBarLoadingText))

	return strings.Join(lines, "\n")
}

// usageSuffixText builds the value text shown after a usage bar. With a limit
// it reads " used/lim (pct%)" (pct of limit, capped at 100). With only a
// request it reads " used/req (pct%)" (pct of request, uncapped so a burst
// above the request shows e.g. 200%). With neither it is just the used value.
func usageSuffixText(used, req, lim int64, formatFn func(int64) string) string {
	switch {
	case lim > 0:
		pct := float64(used) / float64(lim) * 100
		if pct > 100 {
			pct = 100
		}
		return fmt.Sprintf(" %s/%s (%.0f%%)", formatFn(used), formatFn(lim), pct)
	case req > 0:
		pct := float64(used) / float64(req) * 100
		return fmt.Sprintf(" %s/%s (%.0f%%)", formatFn(used), formatFn(req), pct)
	default:
		return " " + formatFn(used)
	}
}

// renderUsageBar renders a single usage bar line. suffixWidth is the reserved
// column width for the value text, so callers can keep multiple bars aligned.
func renderUsageBar(used, req, lim int64, barWidth int, formatFn func(int64) string, suffixWidth int) string {
	if lim <= 0 && req <= 0 {
		// No request and no limit: nothing to scale against, so show the value
		// alone (no bar).
		return NormalStyle.Render(formatFn(used)) + DimStyle.Render(" (no request/limit)")
	}

	suffix := usageSuffixText(used, req, lim, formatFn)

	// Reserve the fixed suffix column so the bar length is independent of the
	// value string width; bars rendered with the same suffixWidth match.
	bw := max(barWidth-suffixWidth-2, 5) // 2 for brackets

	bar := NormalStyle.Render("[") + renderUsageBarFill(bw, used, req, lim) + NormalStyle.Render("]")
	// Right-align the value text within the reserved column so the percentages
	// line up and the line fills the full width.
	return bar + NormalStyle.Render(padLeft(suffix, suffixWidth))
}

// renderUsageBarFill builds the inner bar (between the brackets). The filled
// cells are split into zones by value: green below the request, orange from the
// request up to 90% of the limit, red at/above 90% of the limit. The red zone
// wins regardless of the request, so a container whose request sits near its
// limit still turns red as usage approaches the limit. The remainder is a dim
// headroom track; when usage has not reached the request, a tick marks the
// request position. With a limit the bar spans 0..limit; without one it spans
// 0..max(usage, request) (so an over-request burst is visible) and there is no
// red zone \u2014 there is no hard cap to reach. Glyphs ramp \u2592 -> \u2593 -> \u2588 so the zones
// stay legible without color (NO_COLOR terminals).
func renderUsageBarFill(bw int, used, req, lim int64) string {
	// The bar's full width is the limit when set, otherwise the larger of usage
	// and request (so an over-request burst fills the bar).
	ref := lim
	if ref <= 0 {
		ref = max(used, req)
	}
	if ref <= 0 {
		return styleGlyphs(usageBarEmptyGlyph, bw, ColorBorder)
	}

	usageN := usageBarCells(bw, used, ref)
	// Zone boundaries in cells. Green ends at the request (bw when there is no
	// request, so no orange band). Red starts at 90% of the limit (bw+1 when
	// there is no limit, so no red band). Green never extends past where red
	// begins.
	reqN := bw
	if req > 0 {
		reqN = usageBarCells(bw, req, ref)
	}
	redN := bw + 1
	if lim > 0 {
		redN = usageBarCells(bw, int64(usageBarRedThreshold*float64(lim)), ref)
	}
	greenEnd := min(reqN, redN)

	green := min(usageN, greenEnd)
	red := max(0, usageN-redN)
	orange := usageN - green - red

	var b strings.Builder
	b.WriteString(styleGlyphs(zoneBelowRequest.fillGlyph(), green, zoneBelowRequest.color()))
	b.WriteString(styleGlyphs(zoneAboveRequest.fillGlyph(), orange, zoneAboveRequest.color()))
	b.WriteString(styleGlyphs(zoneNearLimit.fillGlyph(), red, zoneNearLimit.color()))

	switch {
	case req > 0 && reqN > usageN && reqN < bw:
		// Request not reached: tick it in the dim headroom track, colored by the
		// zone usage would enter on crossing it (orange, or red near the limit).
		b.WriteString(styleGlyphs(usageBarEmptyGlyph, reqN-usageN, ColorBorder))
		b.WriteString(styleGlyphs(usageBarRequestTick, 1, usageBarZone(req, req, lim).color()))
		b.WriteString(styleGlyphs(usageBarEmptyGlyph, bw-reqN-1, ColorBorder))
	default:
		b.WriteString(styleGlyphs(usageBarEmptyGlyph, bw-usageN, ColorBorder))
	}
	return b.String()
}

// styleGlyphs renders n copies of glyph in the given foreground color. Returns
// "" for n <= 0.
func styleGlyphs(glyph string, n int, color string) string {
	if n <= 0 {
		return ""
	}
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Background(BaseBg).Render(strings.Repeat(glyph, n))
}

// usageBarCells maps a value to a bar-cell count in [0, bw], rounded.
func usageBarCells(bw int, val, ref int64) int {
	if ref <= 0 || val <= 0 {
		return 0
	}
	c := int(float64(bw)*float64(val)/float64(ref) + 0.5)
	return max(0, min(c, bw))
}

// usageBarEmptyGlyph fills the unused portion of a bar. It is lighter than any
// zone fill glyph so the filled/empty boundary stays visible without color.
const usageBarEmptyGlyph = "\u2591" // U+2591 light shade

// usageBarRedThreshold is the fraction of the limit at (or above) which usage
// is shown red ("near limit").
const usageBarRedThreshold = 0.90

// usageBarRequestTick marks the request position in the dim headroom track when
// usage has not reached the request yet. A full block (so the cell fills like
// the rest of the bar instead of showing the background through a thin glyph),
// colored like the above-request band so it reads as "crossing here turns the
// bar orange". In NO_COLOR terminals it is the only full block in the dim track.
const usageBarRequestTick = "\u2588" // U+2588 full block

// usageZone classifies usage relative to the request and limit.
type usageZone int

const (
	zoneBelowRequest usageZone = iota // under the request
	zoneAboveRequest                  // at/above the request, below the limit
	zoneNearLimit                     // approaching or over the limit
)

// usageBarZone picks the zone: red once usage reaches 90% of the limit (or
// over), orange once it passes the request, green below the request. Falls back
// sensibly when only one of request/limit is set (callers never invoke this
// when both are zero).
func usageBarZone(used, req, lim int64) usageZone {
	if lim > 0 && float64(used) >= usageBarRedThreshold*float64(lim) {
		return zoneNearLimit
	}
	if req > 0 && used >= req {
		return zoneAboveRequest
	}
	return zoneBelowRequest
}

func (z usageZone) color() string {
	switch z {
	case zoneNearLimit:
		return ColorError
	case zoneAboveRequest:
		return ColorOrange
	default:
		return ColorSecondary
	}
}

// fillGlyph returns the filled-cell rune for the zone. It ramps from a light to
// a full block (\u2592 -> \u2593 -> \u2588) so severity reads by density alone, keeping the
// bar legible in NO_COLOR terminals where zone.color() is dropped.
func (z usageZone) fillGlyph() string {
	switch z {
	case zoneNearLimit:
		return "\u2588" // U+2588 full block
	case zoneAboveRequest:
		return "\u2593" // U+2593 dark shade
	default:
		return "\u2592" // U+2592 medium shade
	}
}
