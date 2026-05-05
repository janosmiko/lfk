package ui

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
)

// rightsizingInnerPanelStyle is the bordered panel containing the
// recommendations table. Background fields chained at render time so
// the value comes from the active theme (matches the K/V editor
// overlays' bg-fix pattern).
var rightsizingInnerPanelStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color(ColorBorder)).
	Padding(0, 1)

// RenderRightsizingOverlay paints the right-sizing advisor overlay
// for the given recommendation payload. Loading/empty/error states
// take precedence over data display so the user always sees a
// coherent message.
//
// `scroll` is the visible-row offset within the table when the
// content overflows the panel height. Owned by the handler.
func RenderRightsizingOverlay(data *model.Rightsizing, loading bool, err error, scroll, screenW, screenH int) string {
	boxW := screenW * 75 / 100
	boxH := screenH * 75 / 100
	if boxW < 60 {
		boxW = 60
	}
	if boxH < 12 {
		boxH = 12
	}

	outerPadH := 4
	outerPadW := 6
	innerPadH := 2
	innerPadW := 4
	titleH := 2 // title + header strip
	gapH := 1

	panelContentH := max(boxH-outerPadH-innerPadH-titleH-gapH, 4)
	panelContentW := max(boxW-outerPadW-innerPadW, 30)
	panelW := boxW - outerPadW

	title := OverlayTitleStyle.Background(BaseBg).Render("Right-sizing")

	header := renderRightsizingHeader(data, loading, err)

	var body string
	switch {
	case loading && data == nil:
		// Cold load — no stale data to show. Centered placeholder
		// takes over the panel until the first fetch lands.
		body = centerInPanel("Computing right-sizing…", panelContentW, panelContentH)
	case err != nil:
		body = centerInPanel(ErrorStyle.Render(err.Error()), panelContentW, panelContentH)
	case data == nil || len(data.Containers) == 0:
		msg := "No recommendations: no containers to sample."
		if data != nil && data.PodCount == 0 {
			msg = "No running pods to sample."
		}
		body = centerInPanel(msg, panelContentW, panelContentH)
	default:
		// Default branch covers both `loading=false` and the
		// "fetching new strategy with stale data still on screen"
		// case (loading=true && data != nil). The header (above)
		// signals the in-progress fetch with a "Loading…" suffix so
		// the user sees the strategy switch is still settling
		// without losing their current view.
		body = renderRightsizingTable(data, scroll, panelContentW, panelContentH)
	}

	innerPanel := rightsizingInnerPanelStyle.
		Background(BaseBg).
		BorderBackground(BaseBg).
		Width(panelW).
		Height(panelContentH).
		Render(body)

	full := title + "\n" + header + "\n" + innerPanel
	return OverlayStyle.
		Background(BaseBg).
		BorderBackground(BaseBg).
		Width(boxW).
		Render(full)
}

// renderRightsizingHeader builds the strategy/methodology summary that
// sits above the table. The header shows:
//
//   - Strategy: <human label> [N/M]   — current strategy + position in
//     the available list. The chip is omitted when only one strategy
//     is available so it doesn't promise a non-functional `[/]` cycle.
//   - Headroom: <H>x [N/M]            — current headroom + position in
//     the preset list. Always shown (the preset list always has 6
//     entries, so </> always has somewhere to go).
//   - Pods aggregated: N              — number of pods sampled.
//   - <methodology hint>              — what window / source backs the
//     numbers, with the active headroom appended ("x 1.25 headroom")
//     so the user sees the actual multiplier they selected.
//   - Loading…                        — appended only when the picker
//     kicked a fetch but the previous payload is still on screen
//     (loading=true && data != nil). Cold loads (data==nil) get the
//     centered "Computing right-sizing…" body instead and don't
//     duplicate the indicator here.
func renderRightsizingHeader(data *model.Rightsizing, loading bool, err error) string {
	if loading && data == nil {
		// Cold load — the centered body owns the placeholder; header
		// stays minimal so the cold state reads as one focused message.
		return BarDimStyle.Render("Loading…")
	}
	if err != nil || data == nil {
		return BarDimStyle.Render(" ")
	}
	label := data.Strategy.HumanLabel()
	if label == "" {
		label = data.Source
	}
	if label == "" {
		label = "snapshot"
	}
	strategyText := label
	if chip := strategyPickerChip(data); chip != "" {
		strategyText = label + " " + chip
	}
	parts := []string{
		BarDimStyle.Render("Strategy: ") + BarNormalStyle.Render(strategyText),
	}
	if headroomText := headroomPickerText(data); headroomText != "" {
		parts = append(parts, BarDimStyle.Render("Headroom: ")+BarNormalStyle.Render(headroomText))
	}
	parts = append(parts, BarDimStyle.Render(fmt.Sprintf("Pods aggregated: %d", data.PodCount)))
	if methodology := rightsizingMethodologyHint(data); methodology != "" {
		parts = append(parts, BarDimStyle.Render(methodology))
	}
	if loading {
		// Stale-data-with-fetch path: a quiet "Loading…" chip tells
		// the user the new strategy is on its way without wiping the
		// table they're looking at.
		parts = append(parts, BarDimStyle.Render("Loading…"))
	}
	return strings.Join(parts, "    ")
}

// strategyPickerChip returns the "[N/M]" chip showing the active
// strategy's position in the available list. Returns "" when fewer
// than 2 strategies are available — the chip is noise without a
// meaningful `[/]` cycle to navigate.
func strategyPickerChip(data *model.Rightsizing) string {
	if data == nil || len(data.AvailableStrategies) < 2 {
		return ""
	}
	for i, s := range data.AvailableStrategies {
		if s == data.Strategy {
			return fmt.Sprintf("[%d/%d]", i+1, len(data.AvailableStrategies))
		}
	}
	return ""
}

// headroomPickerText returns "<H>x [N/M]" — value followed by the
// position chip. Always rendered when Headroom is set, since
// model.RightsizingHeadrooms is a fixed 6-entry list and `</>` always
// has somewhere to go.
//
// `%g` formats whole numbers without trailing zeros ("2x") and
// fractional values with their natural precision ("1.25x").
func headroomPickerText(data *model.Rightsizing) string {
	if data == nil || data.Headroom == 0 {
		return ""
	}
	value := fmt.Sprintf("%gx", data.Headroom)
	if chip := headroomPickerChip(data.Headroom); chip != "" {
		return value + " " + chip
	}
	return value
}

// headroomPickerChip returns the "[N/M]" position chip for the
// active headroom in model.RightsizingHeadrooms. Returns "" when the
// active headroom doesn't match any preset (caller hides the chip
// rather than printing "?").
func headroomPickerChip(h float64) string {
	idx := headroomIndex(h)
	if idx < 0 {
		return ""
	}
	return fmt.Sprintf("[%d/%d]", idx+1, len(model.RightsizingHeadrooms))
}

// headroomIndex returns the position of `h` in
// model.RightsizingHeadrooms, or -1 if it doesn't match any preset.
// Epsilon comparison so a value that round-tripped through %g / parse
// stays matchable.
func headroomIndex(h float64) int {
	for i, v := range model.RightsizingHeadrooms {
		if math.Abs(v-h) < 1e-9 {
			return i
		}
	}
	return -1
}

// rightsizingMethodologyHint returns a short, human-readable string
// describing how the recommendations were computed. Prefers the
// strategy's MethodologyHint (covers VPA / Prometheus windows /
// snapshot uniformly) and appends the active Headroom so the user
// sees the actual multiplier ("x 1.25 headroom"). Falls back to
// legacy Source-string matching for callers that don't set Strategy.
func rightsizingMethodologyHint(data *model.Rightsizing) string {
	if data == nil {
		return ""
	}
	if data.Strategy != "" {
		hint := data.Strategy.MethodologyHint()
		if data.Strategy == model.StrategySnapshot && data.Window != "" {
			hint += " (window: " + data.Window + ")"
		}
		hint += headroomMethodologySuffix(data.Strategy, data.Headroom)
		return hint
	}
	// Legacy fallback (used by older fixtures + external callers that
	// only set Source/Window).
	switch data.Source {
	case "VPA":
		return "VPA recommender (history-based)"
	case "estimated":
		hint := "current usage x 1.2 headroom"
		if data.Window != "" {
			hint += " (window: " + data.Window + ")"
		}
		return hint
	}
	return ""
}

// headroomMethodologySuffix is the trailing "x <H> headroom" appended
// to the strategy-only methodology hint. For VPA at headroom 1.0 the
// suffix is "(raw)" instead so the user understands the recommendation
// is the recommender's untouched output (no padding) — printing
// "x 1 headroom" reads as redundant noise.
//
// Always returns a leading space so the caller can splice the suffix
// directly without extra spacing logic.
func headroomMethodologySuffix(s model.RightsizingStrategy, headroom float64) string {
	if headroom == 0 {
		return ""
	}
	if s == model.StrategyVPA && headroom == 1.0 {
		return " (raw)"
	}
	return fmt.Sprintf(" x %g headroom", headroom)
}

// renderRightsizingTable draws the 2-row grouped-header recommendations
// table sized to fill `width`. The top header row has REQUEST / LIMIT
// (and BOUNDS for VPA) group spans whose cells render centered without
// internal separators inside each span; the sub-header row breaks each
// group into CURRENT / SUGGESTION / Δ sub-columns. The BOUNDS group is
// dropped when source != VPA, since estimated recommendations carry no
// lower/upper bounds.
func renderRightsizingTable(data *model.Rightsizing, scroll, width, height int) string {
	hasBounds := data != nil && data.Source == "VPA"
	layout := rsLayoutFor(hasBounds, width)

	// Reserve 3 lines for header (group + sub-header + mid-rule).
	maxRows := max(height-3, 0)
	rows := buildRightsizingDataRows(data, scroll, maxRows, hasBounds, layout)

	var b strings.Builder
	b.WriteString(renderRSGroupHeader(layout))
	b.WriteByte('\n')
	b.WriteString(renderRSSubHeader(layout))
	b.WriteByte('\n')
	b.WriteString(renderRSSeparator(layout))
	for _, r := range rows {
		b.WriteByte('\n')
		b.WriteString(renderRSDataRow(layout, r))
	}
	return b.String()
}

// buildRightsizingDataRows yields one pre-rendered data row per
// (container, resource). `scroll` skips the first N rows; `maxRows`
// clips the rest. Each row is a slice of styled cell strings (already
// padded + width-fitted) ready to be joined by the row renderer.
func buildRightsizingDataRows(
	data *model.Rightsizing,
	scroll, maxRows int,
	hasBounds bool,
	layout rsLayout,
) [][]string {
	if data == nil {
		return nil
	}
	all := make([][]string, 0, len(data.Containers)*2)
	for _, c := range data.Containers {
		all = append(all, rightsizingDataRow(c.Name, "cpu", c.CPU, hasBounds, layout))
		all = append(all, rightsizingDataRow(c.Name, "mem", c.Mem, hasBounds, layout))
	}
	if scroll < 0 {
		scroll = 0
	}
	if scroll > len(all) {
		scroll = len(all)
	}
	end := min(scroll+maxRows, len(all))
	return all[scroll:end]
}

// rightsizingDataRow renders one (container, resource) row. Returns a
// slice of pre-styled cell strings, one per sub-column in `layout`.
// Cell ordering: CONTAINER, RES, USAGE, REQ.CUR, REQ.SUGG, REQ.Δ,
// LIM.CUR, LIM.SUGG, LIM.Δ, [BND.LOWER, BND.UPPER].
func rightsizingDataRow(
	name, resKey string,
	r model.ResourceRec,
	hasBounds bool,
	layout rsLayout,
) []string {
	cells := []string{
		renderRSCell(truncateName(name, layout.widths[0]), layout.widths[0], layout.aligns[0], BarNormalStyle),
		renderRSCell(resKey, layout.widths[1], layout.aligns[1], BarNormalStyle),
		valueCellOrDash(r.Usage, layout.widths[2], layout.aligns[2]),
		currentCell(r.CurrentRequest, layout.widths[3], layout.aligns[3]),
		suggestionCell(r.RecommendedRequest, layout.widths[4], layout.aligns[4]),
		deltaCell(r.CurrentRequest, r.RecommendedRequest, layout.widths[5], layout.aligns[5]),
		currentCell(r.CurrentLimit, layout.widths[6], layout.aligns[6]),
		suggestionCell(r.RecommendedLimit, layout.widths[7], layout.aligns[7]),
		deltaCell(r.CurrentLimit, r.RecommendedLimit, layout.widths[8], layout.aligns[8]),
	}
	if hasBounds {
		cells = append(cells,
			valueCellOrDash(r.LowerBound, layout.widths[9], layout.aligns[9]),
			valueCellOrDash(r.UpperBound, layout.widths[10], layout.aligns[10]),
		)
	}
	return cells
}

// valueCellOrDash renders a value cell — `BarNormalStyle` when set, dim
// em-dash placeholder when empty. Coordinating the placeholder with the
// cell's base style HERE (rather than passing pre-styled `orDash`
// content into `renderRSCell`) keeps the content plain so `rsTruncate`
// inside the cell renderer can do rune-based truncation safely. The old
// `orDash` returned an ANSI-styled string whose escape bytes counted as
// runes, so any cell wider than the escape sequence got cut mid-escape
// and the `—` never reached the screen.
func valueCellOrDash(v string, width int, align lipgloss.Position) string {
	if v == "" {
		return renderRSCell("—", width, align, BarDimStyle)
	}
	return renderRSCell(v, width, align, BarNormalStyle)
}

// currentCell renders the CURRENT column of a request/limit pair. Uses
// BarDimStyle so the suggestion column reads as the primary signal —
// the user's eye lands on what they should change TO, not on what they
// have. Empty current → dim em-dash so the absence is visible.
func currentCell(current string, width int, align lipgloss.Position) string {
	if current == "" {
		return renderRSCell("—", width, align, BarDimStyle)
	}
	return renderRSCell(current, width, align, BarDimStyle)
}

// suggestionCell renders the SUGGESTION column. Rendered through
// BarNormalStyle so it visually anchors the row. Empty suggestion →
// dim em-dash to match the current-column placeholder.
func suggestionCell(recommended string, width int, align lipgloss.Position) string {
	if recommended == "" {
		return renderRSCell("—", width, align, BarDimStyle)
	}
	return renderRSCell(recommended, width, align, BarNormalStyle)
}

// deltaCell renders the Δ column (signed percentage change). Returns a
// dim "—" when either side is missing (no comparison possible) and a
// dim "=" when the values are equal. Otherwise renderDelta colours the
// change by direction and magnitude.
func deltaCell(current, recommended string, width int, align lipgloss.Position) string {
	if current == "" || recommended == "" {
		return renderRSCell("—", width, align, BarDimStyle)
	}
	if current == recommended {
		return renderRSCell("=", width, align, BarDimStyle)
	}
	// renderDelta returns an already-styled string. Wrap it in a width-
	// fitted, right-aligned cell with the panel background so widths and
	// background match the surrounding cells.
	pct, ok := DeltaPercent(current, recommended)
	if !ok {
		return renderRSCell("—", width, align, BarDimStyle)
	}
	return renderRSCellPrestyled(formatPct(pct), width, align, deltaForegroundFor(pct))
}

// truncateName clips an over-long container name to the assigned
// column content width so it doesn't overflow into the next cell.
// Padding(0,1) gives the cell 2 chars of internal padding, so the
// effective text room is `nameW - 2`. Adds a trailing ellipsis when
// truncation occurs so the user knows the value is partial. Guards
// against pathologically narrow widths so we never panic.
func truncateName(name string, nameW int) string {
	usable := nameW
	if usable <= 1 {
		return name
	}
	if len([]rune(name)) <= usable {
		return name
	}
	runes := []rune(name)
	return string(runes[:usable-1]) + "…"
}

// deltaForegroundFor maps a percent change to a foreground colour.
//
//   - pct == 0          → dim (no change)
//   - |pct| < 10        → dim (within noise floor)
//   - pct < 0           → ColorSecondary (over-provisioned, "saves money")
//   - pct >= 10         → ColorWarning (under-provisioned)
func deltaForegroundFor(pct float64) string {
	switch {
	case pct == 0:
		return ""
	case math.Abs(pct) < 10:
		return ""
	case pct < 0:
		return ColorSecondary
	default:
		return ColorWarning
	}
}

func formatPct(pct float64) string {
	sign := ""
	if pct > 0 {
		sign = "+"
	}
	return fmt.Sprintf("%s%d%%", sign, int(math.Round(pct)))
}

func centerInPanel(text string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, text,
		lipgloss.WithWhitespaceBackground(BaseBg),
		lipgloss.WithWhitespaceForeground(BaseBg))
}
