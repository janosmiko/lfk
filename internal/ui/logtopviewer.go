package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogTopRow is one display row for the Log Top aggregation table.
type LogTopRow struct {
	Dims     map[string]string
	Count    int
	ErrCount int
	Pct      float64
	P95      float64 // approximate p95 latency in ms; -1 when no duration data
	P99      float64 // approximate p99 latency in ms; -1 when no duration data
}

// dimWeight returns the column weight for a given dimension name.
// path and router/host get extra weight to avoid truncation of long values.
func dimWeight(d string) int {
	switch d {
	case "path":
		return 3
	case "router", "host":
		return 2
	default:
		return 1
	}
}

// logTopColWidths computes column widths for the dim columns.
// dimsRegion is the total character budget; D is the number of dims.
// Returns a slice of widths summing to at most dimsRegion.
func logTopColWidths(dims []string, dimsRegion int) []int {
	d := len(dims)
	if d == 0 {
		return nil
	}
	// The separating spaces between columns take (D-1) chars from the budget.
	// Use the available region minus separators when positive; otherwise floor at
	// d*4 so Truncate can handle the narrow case without widths exceeding dimsRegion.
	var budget int
	if rem := dimsRegion - (d - 1); rem > 0 {
		budget = rem
	} else {
		budget = d * 4
	}
	totalWeight := 0
	for _, dim := range dims {
		totalWeight += dimWeight(dim)
	}
	widths := make([]int, d)
	used := 0
	for i, dim := range dims {
		w := max(dimWeight(dim)*budget/totalWeight, 4)
		widths[i] = w
		used += w
	}
	// Adjust last column to absorb rounding differences; floor at 4.
	widths[d-1] = max(widths[d-1]+budget-used, 4)
	return widths
}

// latencyCell formats a latency value for display in a column of width w.
// Returns "n/a" right-aligned when v < 0, otherwise integer ms right-aligned.
func latencyCell(v float64, w int) string {
	var label string
	if v < 0 {
		label = "n/a"
	} else {
		label = fmt.Sprintf("%.0f", v)
	}
	pad := max(w-len(label), 0)
	return strings.Repeat(" ", pad) + label
}

// RenderLogTopView renders the Log Top aggregation table full screen.
// dims lists all dimension columns to show (in order).
// reqPerSec is the global request rate; total is the total request count used
// to compute per-row REQ/s as a proportional share of the global rate.
// showLatency adds P95 and P99 latency columns when true.
// Call sites must set ui.ActiveSortColumnName and ui.ActiveSortAscending before
// calling so the active sort column is highlighted.
func RenderLogTopView(title string, dims []string, rows []LogTopRow, reqPerSec float64, total, cursor, scroll int, hintBar string, width, height int, showLatency bool) string {
	titleBar := ViewTitle(width, title)

	// Lines must fit inside the bordered box: FullscreenBorderStyle renders at
	// width-2 with 1-col padding each side, leaving width-4 of text. Target
	// width-5 (matching the event viewer) so a column never wraps.
	// Layout: lead(2) + dimsRegion + metricsBlock
	// metricsBlock = 32 base; +16 when showLatency (two " %7s" columns = 2*(1+7)=16)
	innerWidth := max(width-5, 20)
	const lead = 2
	const metricsBlockBase = 32 // " %8d %8.1f %6.1f %6d" = 1+8+1+8+1+6+1+6 = 32
	const latencyExtra = 16     // 2 * (1 space + 7 wide column)
	metricsBlock := metricsBlockBase
	if showLatency {
		metricsBlock += latencyExtra
	}
	dimsRegion := max(innerWidth-lead-metricsBlock, len(dims)*5)
	colWidths := logTopColWidths(dims, dimsRegion)

	// Build header using renderStyledHeader so the active sort column is highlighted.
	var segments []headerSegment
	// leading spaces - no column name
	segments = append(segments, headerSegment{text: strings.Repeat(" ", lead)})
	for i, d := range dims {
		label := strings.ToUpper(d) + sortIndicatorForColumn(d)
		raw := Truncate(label, colWidths[i])
		padded := raw + strings.Repeat(" ", max(colWidths[i]-lipgloss.Width(raw), 0))
		if i > 0 {
			segments = append(segments, headerSegment{text: " "})
		}
		segments = append(segments, headerSegment{text: padded, colName: d})
	}
	// metric columns: right-aligned within their widths, preceded by a space
	metricNames := []string{"REQ", "REQ/s", "%", "ERR"}
	metricWidths := []int{8, 8, 6, 6}
	if showLatency {
		metricNames = append(metricNames, "P95", "P99")
		metricWidths = append(metricWidths, 7, 7)
	}
	for i, name := range metricNames {
		w := metricWidths[i]
		ind := sortIndicatorForColumn(name)
		label := name + ind
		rawW := lipgloss.Width(label)
		pad := max(w-rawW, 0)
		// " " + spaces + label totals 1+w chars
		cell := " " + strings.Repeat(" ", pad) + label
		segments = append(segments, headerSegment{text: cell, colName: name})
	}
	head := renderStyledHeader(segments, lead+dimsRegion+metricsBlock)

	maxRows := max(height-5, 3)
	var b strings.Builder
	b.WriteString(head)
	b.WriteByte('\n')

	end := min(scroll+maxRows, len(rows))
	for i := scroll; i < end; i++ {
		r := rows[i]
		rowRPS := 0.0
		if total > 0 {
			rowRPS = reqPerSec * float64(r.Count) / float64(total)
		}

		dimParts := make([]string, 0, len(dims))
		for j, d := range dims {
			val := r.Dims[d]
			raw := Truncate(val, colWidths[j])
			padded := raw + strings.Repeat(" ", max(colWidths[j]-lipgloss.Width(raw), 0))
			dimParts = append(dimParts, padded)
		}
		dimCols := strings.Join(dimParts, " ")
		metrics := fmt.Sprintf(" %8d %8.1f %6.1f %6d", r.Count, rowRPS, r.Pct, r.ErrCount)
		if showLatency {
			metrics += " " + latencyCell(r.P95, 7) + " " + latencyCell(r.P99, 7)
		}
		line := strings.Repeat(" ", lead) + dimCols + metrics
		if i == cursor {
			line = SelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for n := end - scroll; n < maxRows; n++ {
		b.WriteByte('\n')
	}

	// Content is the header row plus maxRows data rows; size the border to that
	// full height so the view fills the screen exactly (no blank line below the
	// hint bar) and the box doesn't grow past its budget.
	body := FullscreenBorderStyle(width, maxRows+1).Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, hintBar)
}
