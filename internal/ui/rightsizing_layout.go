package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// rsLayout describes the per-row column geometry for the right-sizing
// overlay. `widths` and `aligns` are per sub-column (one entry per
// physical cell). `subHdrs` is the bottom-row header text per
// sub-column. `spans` is the top-row group header (REQUEST, LIMIT,
// BOUNDS); singletons (CONTAINER, RES, USAGE) are NOT included in
// `spans` — they render as blank cells in the group header row.
type rsLayout struct {
	widths  []int
	aligns  []lipgloss.Position
	subHdrs []string
	spans   []rsGroupSpan
}

// rsGroupSpan describes a single group header that spans one or more
// sub-columns. `from` and `to` are sub-column indices (inclusive) into
// `rsLayout.widths`. `name` is the centered group label (e.g. "REQUEST").
type rsGroupSpan struct {
	name     string
	from, to int
}

// Sub-column indices used throughout the layout/render code. Kept as
// constants so the call sites read declaratively instead of relying on
// magic numbers that drift when the layout changes.
const (
	rsColContainer = 0
	rsColRes       = 1
	rsColUsage     = 2
	rsColReqCur    = 3
	rsColReqSugg   = 4
	rsColReqDelta  = 5
	rsColLimCur    = 6
	rsColLimSugg   = 7
	rsColLimDelta  = 8
	rsColBndLower  = 9
	rsColBndUpper  = 10
)

// Per-cell padding chars (left + right). Each rendered cell is
// `" " + content_padded_to_width + " "`, so display width = content
// width + 2.
const rsCellPadding = 2

// Inter-cell separator (1 char wide). The whole table uses the same
// separator string so width math is uniform.
const rsSep = "│"

// Mid-rule character set (used in renderRSSeparator).
const (
	rsMidLine     = "─"
	rsMidJunction = "┼"
)

// Minimum content widths per logical column type. Sized to fit the
// longest sub-header label AND the typical maximum data value so cells
// never wrap (lipgloss `Width(n)` wraps overflow rather than truncating).
// CONTAINER stays near header width because names are variable and get
// ellipsised when over-long; value columns must accommodate "SUGGESTION"
// (10) so the bottom-row labels render whole. USAGE has to fit memory
// values like "2162Mi" (6) and "10000Mi" (7) — bumped to 8 so we don't
// wrap on real argo-cd / kafka workloads. Δ holds "-100%" (5 chars).
// Bound columns hold the LOWER/UPPER labels (5 chars).
const (
	rsMinContainer = 9  // "CONTAINER"
	rsMinRes       = 3  // "RES"
	rsMinUsage     = 8  // "USAGE" / "10000Mi" — wide enough for typical memory values
	rsMinValueCol  = 10 // "SUGGESTION" — longest sub-header
	rsMinDelta     = 5  // "-100%" / "Δ" header is 1 char but data fits 5
	rsMinBoundCol  = 5  // "LOWER" / "UPPER"
)

// rsLayoutFor returns the column layout that fits in `panelW` total
// display columns. When hasBounds is true the layout includes the
// BOUNDS group (LOWER + UPPER sub-columns), otherwise it stops after
// LIMIT. Any rounding remainder lands in the last variable column so
// the row width sums exactly to `panelW`.
//
// Width math (per row):
//
//	total = sum(widths[i]) + 2*N + (N-1)
//	      = sum(widths[i]) + 3N - 1
//
// where N is the number of sub-columns. The +2 per cell is the cell
// padding; the (N-1) accounts for inter-cell separators.
func rsLayoutFor(hasBounds bool, panelW int) rsLayout {
	subHdrs := []string{"CONTAINER", "RES", "USAGE", "CURRENT", "SUGGESTION", "Δ", "CURRENT", "SUGGESTION", "Δ"}
	aligns := []lipgloss.Position{
		lipgloss.Left,  // CONTAINER
		lipgloss.Left,  // RES
		lipgloss.Right, // USAGE
		lipgloss.Right, // REQ.CUR
		lipgloss.Right, // REQ.SUGG
		lipgloss.Right, // REQ.Δ
		lipgloss.Right, // LIM.CUR
		lipgloss.Right, // LIM.SUGG
		lipgloss.Right, // LIM.Δ
	}
	mins := []int{
		rsMinContainer,
		rsMinRes,
		rsMinUsage,
		rsMinValueCol,
		rsMinValueCol,
		rsMinDelta,
		rsMinValueCol,
		rsMinValueCol,
		rsMinDelta,
	}
	spans := []rsGroupSpan{
		{name: "REQUEST", from: rsColReqCur, to: rsColReqDelta},
		{name: "LIMIT", from: rsColLimCur, to: rsColLimDelta},
	}
	if hasBounds {
		subHdrs = append(subHdrs, "LOWER", "UPPER")
		aligns = append(aligns, lipgloss.Right, lipgloss.Right)
		mins = append(mins, rsMinBoundCol, rsMinBoundCol)
		spans = append(spans, rsGroupSpan{name: "BOUNDS", from: rsColBndLower, to: rsColBndUpper})
	}

	widths := rsDistributeWidths(panelW, mins)
	return rsLayout{
		widths:  widths,
		aligns:  aligns,
		subHdrs: subHdrs,
		spans:   spans,
	}
}

// rsDistributeWidths assigns content widths to each sub-column so the
// row's total display width equals `panelW` INCLUDING the trailing
// closing separator that every row renderer appends (`│` for sub-header
// + data, `┤` for mid-rule, `┐` for the group header). Keeps the
// fixed-content columns (CONTAINER, RES, USAGE) at their minimums and
// pours all surplus into CONTAINER + the value columns proportionally:
//
//   - CONTAINER absorbs ~25% of surplus (variable-length names benefit
//     most from extra width).
//   - Each CURRENT/SUGGESTION/LOWER/UPPER column gets the same share.
//   - Δ columns stay narrow (they always render a 4-5 char string).
//   - Rounding remainder lands in the last variable column.
//
// When `panelW` is below the floor (sum of mins + chrome) the function
// returns the minimums as-is — caller renders, lipgloss truncates.
func rsDistributeWidths(panelW int, mins []int) []int {
	n := len(mins)
	widths := make([]int, n)
	copy(widths, mins)

	// Chrome = 2 padding chars per cell + (n-1) inter-cell separators
	// + 1 trailing closing separator that all row renderers append.
	chrome := 2*n + (n - 1) + 1
	floor := chrome
	for _, m := range mins {
		floor += m
	}
	extra := panelW - floor
	if extra <= 0 {
		return widths
	}

	// Variable columns are CONTAINER (idx 0) + every value column
	// (CURRENT / SUGGESTION / LOWER / UPPER). Δ columns and the fixed
	// RES / USAGE columns don't grow.
	growable := []int{rsColContainer, rsColReqCur, rsColReqSugg, rsColLimCur, rsColLimSugg}
	if n > 9 {
		growable = append(growable, rsColBndLower, rsColBndUpper)
	}
	// CONTAINER gets weight 3 (variable-length names benefit most),
	// each value column gets weight 2.
	weights := make([]int, len(growable))
	for i, col := range growable {
		if col == rsColContainer {
			weights[i] = 3
		} else {
			weights[i] = 2
		}
	}
	totalW := 0
	for _, w := range weights {
		totalW += w
	}
	used := 0
	for i, col := range growable {
		add := extra * weights[i] / totalW
		widths[col] += add
		used += add
	}
	// Drop the rounding remainder into CONTAINER so the total is exact.
	widths[rsColContainer] += extra - used
	return widths
}

// renderRSGroupHeader renders the top header row. Singletons (the
// columns NOT covered by any span) render as blank space; group spans
// render their `name` centered within the span's display width with
// `─` filler chars on both sides so the span's extent is visually
// obvious. Separators between cells use box-drawing corners to bracket
// each group:
//
//   - singleton → singleton: blank space (no border, since neither cell
//     belongs to a group)
//   - singleton → group: `┌` (opening bracket of a group)
//   - group → group: `┬` (closing previous + opening next)
//   - group → singleton: `┐` (closing bracket)
//
// After the last cell, the row appends a trailing closing char (`┐`
// if the last cell is a group, otherwise a blank space) so the group
// header row has the same total width as the sub-header + data rows
// (which both append `│`). The chrome calc in `rsDistributeWidths`
// reserves the +1 char for this trailing slot.
//
// Corners + filler use `ColorBorder` to match the mid-rule + sub-header
// separators; the group `name` itself uses `ColorPrimary` bold so it
// reads as the dominant signal.
func renderRSGroupHeader(layout rsLayout) string {
	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorPrimary)).
		Background(BaseBg).
		Bold(true)
	borderStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBorder)).
		Background(BaseBg)
	blankStyle := lipgloss.NewStyle().Background(BaseBg)

	var b strings.Builder
	i := 0
	first := true
	prevWasGroup := false
	for i < len(layout.widths) {
		span, isSpan := rsSpanContaining(layout.spans, i)

		if !first {
			switch {
			case !prevWasGroup && !isSpan:
				b.WriteString(blankStyle.Render(" "))
			case !prevWasGroup && isSpan:
				b.WriteString(borderStyle.Render("┌"))
			case prevWasGroup && isSpan:
				b.WriteString(borderStyle.Render("┬"))
			case prevWasGroup && !isSpan:
				b.WriteString(borderStyle.Render("┐"))
			}
		}
		first = false

		if isSpan {
			displayW := rsSpanDisplayWidth(layout, span)
			b.WriteString(renderRSGroupSpanContent(span.name, displayW, nameStyle, borderStyle))
			i = span.to + 1
			prevWasGroup = true
		} else {
			displayW := layout.widths[i] + rsCellPadding
			b.WriteString(blankStyle.Render(strings.Repeat(" ", displayW)))
			i++
			prevWasGroup = false
		}
	}
	// Trailing closing slot — `┐` after the last group, blank otherwise.
	if prevWasGroup {
		b.WriteString(borderStyle.Render("┐"))
	} else {
		b.WriteString(blankStyle.Render(" "))
	}
	return b.String()
}

// renderRSGroupSpanContent draws "─── NAME ───" inside a span of the
// given total `width`. `─` filler uses `borderStyle`, the `NAME` token
// (with surrounding 1-char spaces for breathing room) uses `nameStyle`.
// When the name doesn't fit the span, the function falls back to
// centered name with no filler so the cell still aligns to width.
func renderRSGroupSpanContent(name string, width int, nameStyle, borderStyle lipgloss.Style) string {
	inner := " " + name + " "
	innerW := len([]rune(inner))
	if innerW >= width {
		// Name (with surrounding pad) is already at/over span width —
		// fall back to centered name with no filler. We still apply
		// `Width(width)` so the cell pads/truncates to the exact span.
		return nameStyle.Width(width).Align(lipgloss.Center).Render(name)
	}
	pad := width - innerW
	leftPad := pad / 2
	rightPad := pad - leftPad
	return borderStyle.Render(strings.Repeat("─", leftPad)) +
		nameStyle.Render(inner) +
		borderStyle.Render(strings.Repeat("─", rightPad))
}

// renderRSSubHeader renders the bottom header row. Every sub-column
// gets its own labeled cell, separated by `│`, with a trailing `│` on
// the right edge so the table has a closed right border.
func renderRSSubHeader(layout rsLayout) string {
	hdrStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorPrimary)).
		Background(BaseBg).
		Bold(true).
		Underline(true)
	cells := make([]string, len(layout.widths))
	for i, hdr := range layout.subHdrs {
		cells[i] = renderRSCell(hdr, layout.widths[i], layout.aligns[i], hdrStyle)
	}
	sep := separatorChar()
	return strings.Join(cells, sep) + sep
}

// renderRSSeparator renders the mid-rule between the sub-header row
// and the data rows. `─` for cell content, `┼` at every internal
// separator position, and `┤` at the right edge so the rule terminates
// cleanly under the closing `│` of the sub-header / data rows above
// and below.
func renderRSSeparator(layout rsLayout) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBorder)).
		Background(BaseBg)
	parts := make([]string, len(layout.widths))
	for i, w := range layout.widths {
		parts[i] = strings.Repeat(rsMidLine, w+rsCellPadding)
	}
	return style.Render(strings.Join(parts, rsMidJunction) + "┤")
}

// renderRSDataRow assembles a data row from pre-rendered cells. Cells
// must be pre-styled and width-fitted (use `renderRSCell` /
// `renderRSCellPrestyled`); this helper just inserts separators
// between them and appends the trailing closing `│`. Layout is unused
// here — kept on the signature so all row renderers share a uniform
// shape (group/sub/sep/data).
func renderRSDataRow(_ rsLayout, cells []string) string {
	sep := separatorChar()
	return strings.Join(cells, sep) + sep
}

// separatorChar returns the inter-cell separator with the panel
// background applied so the divider doesn't show as a transparent
// gap on terminals that distinguish bg from default.
func separatorChar() string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBorder)).
		Background(BaseBg).
		Render(rsSep)
}

// renderRSCell renders a single cell with the given content, content
// width, alignment, and base style. Adds Padding(0, 1) so the cell's
// total display width = content width + 2. Content longer than `width`
// is hard-truncated with `…` so the cell can never wrap and break
// row alignment (lipgloss's default `Width(n)` wraps overflow).
func renderRSCell(content string, width int, align lipgloss.Position, base lipgloss.Style) string {
	return base.
		Padding(0, 1).
		Background(BaseBg).
		Width(width + rsCellPadding).
		Align(align).
		Render(rsTruncate(content, width))
}

// renderRSCellPrestyled is the variant for cells whose content is a
// single colour (not a base lipgloss style). Lets callers pass an
// empty `fg` to mean "dim".
func renderRSCellPrestyled(content string, width int, align lipgloss.Position, fg string) string {
	style := lipgloss.NewStyle().
		Padding(0, 1).
		Background(BaseBg).
		Width(width + rsCellPadding).
		Align(align)
	if fg == "" {
		// Empty fg → dim (matches BarDimStyle treatment).
		style = style.Faint(true)
	} else {
		style = style.Foreground(lipgloss.Color(fg))
	}
	return style.Render(rsTruncate(content, width))
}

// rsTruncate cuts `s` to at most `width` runes. When `s` exceeds the
// budget the last char becomes `…` so the truncation is visible. When
// `width <= 0` returns the empty string. Plain runes only — assumes
// content has no embedded ANSI sequences (data values are unstyled
// strings; the styling lives on the surrounding `renderRSCell` style).
func rsTruncate(s string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= width {
		return s
	}
	if width == 1 {
		return "…"
	}
	return string(runes[:width-1]) + "…"
}

// rsSpanContaining returns the span that contains sub-column index
// `i`, or zero + false if none does.
func rsSpanContaining(spans []rsGroupSpan, i int) (rsGroupSpan, bool) {
	for _, s := range spans {
		if i >= s.from && i <= s.to {
			return s, true
		}
	}
	return rsGroupSpan{}, false
}

// rsSpanDisplayWidth returns the total display width of a span,
// including its sub-column cell padding plus the separators that
// would have appeared inside the span if it weren't a span.
func rsSpanDisplayWidth(layout rsLayout, span rsGroupSpan) int {
	w := 0
	for i := span.from; i <= span.to; i++ {
		w += layout.widths[i] + rsCellPadding
	}
	// (to-from) inter-cell separators are absorbed into the span.
	w += span.to - span.from
	return w
}
