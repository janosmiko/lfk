package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

// logTopDrillMarker prefixes the header of the dimension column that the next
// drill-down (Enter) will expand into.
const logTopDrillMarker = "▸" // ▸

// ActiveLogTopNextDrill names the dimension column the next Log Top drill-down
// will expand into. Set by the caller before RenderLogTopView; "" hides the
// marker. Follows the same "set ui.Active* before render" pattern as
// ActiveSortColumnName.
var ActiveLogTopNextDrill string

// LogTopRow is one display row for the Log Top aggregation table.
type LogTopRow struct {
	Dims      map[string]string
	Count     int
	ErrCount  int
	Pct       float64
	P50       float64 // approximate p50 latency in ms; -1 when no duration data
	P95       float64 // approximate p95 latency in ms; -1 when no duration data
	P99       float64 // approximate p99 latency in ms; -1 when no duration data
	Avg       float64 // mean latency in ms; -1 when no duration data
	Max       float64 // max latency in ms; -1 when no duration data
	Status4xx int     // count of HTTP 4xx responses
	Status5xx int     // count of HTTP 5xx responses
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
	pad := max(w-lipgloss.Width(label), 0)
	return strings.Repeat(" ", pad) + label
}

// metricWidth returns the column character width for a given metric id.
func metricWidth(id string) int {
	switch id {
	case "%", "ERR", "ERR%", "4XX", "5XX":
		return 6
	case "P50", "P95", "P99", "AVG", "MAX":
		return 7
	default: // REQ, REQ/s
		return 8
	}
}

// metricCell formats a single metric value for a row, right-aligned within its column width.
func metricCell(id string, r LogTopRow, rowRPS float64) string {
	var label string
	switch id {
	case "REQ":
		label = fmt.Sprintf("%d", r.Count)
	case "REQ/s":
		label = fmt.Sprintf("%.1f", rowRPS)
	case "%":
		label = fmt.Sprintf("%.1f", r.Pct)
	case "ERR":
		label = fmt.Sprintf("%d", r.ErrCount)
	case "ERR%":
		rate := 0.0
		if r.Count > 0 {
			rate = 100 * float64(r.ErrCount) / float64(r.Count)
		}
		label = fmt.Sprintf("%.1f", rate)
	case "4XX":
		label = fmt.Sprintf("%d", r.Status4xx)
	case "5XX":
		label = fmt.Sprintf("%d", r.Status5xx)
	case "AVG":
		return latencyCell(r.Avg, metricWidth(id))
	case "MAX":
		return latencyCell(r.Max, metricWidth(id))
	case "P50":
		return latencyCell(r.P50, metricWidth(id))
	case "P95":
		return latencyCell(r.P95, metricWidth(id))
	case "P99":
		return latencyCell(r.P99, metricWidth(id))
	}
	w := metricWidth(id)
	pad := max(w-lipgloss.Width(label), 0)
	return strings.Repeat(" ", pad) + label
}

// RenderLogTopView renders the Log Top aggregation table full screen.
// dims lists the visible dimension columns to show (in order).
// metrics lists the visible metric column ids to show (in order).
// reqPerSec is the global request rate; total is the total request count used
// to compute per-row REQ/s as a proportional share of the global rate.
// Call sites must set ui.ActiveSortColumnName and ui.ActiveSortAscending before
// calling so the active sort column is highlighted.
func RenderLogTopView(title string, dims []string, metrics []string, rows []LogTopRow, reqPerSec float64, total, cursor, scroll int, hintBar string, width, height int) string {
	// Work on a local copy so the overflow-drop loop does not mutate the caller's slice.
	metrics = append([]string(nil), metrics...)

	titleBar := ViewTitle(width, title)

	// Lines must fit inside the bordered box: FullscreenBorderStyle renders at
	// width-2 with 1-col padding each side, leaving width-4 of text. Target
	// width-5 (matching the event viewer) so a column never wraps.
	// Layout: lead(2) + dimsRegion + metricsBlock
	// metricsBlock = sum of (1+metricWidth(id)) for each visible metric
	innerWidth := max(width-5, 20)
	const lead = 2
	metricsBlock := 0
	for _, id := range metrics {
		metricsBlock += 1 + metricWidth(id)
	}

	// Overflow-drop: if metrics block leaves dims region too narrow, drop
	// right-most metrics until dims have enough room (at least 4 chars per dim).
	for len(metrics) > 1 && innerWidth-lead-metricsBlock < len(dims)*4 {
		metrics = metrics[:len(metrics)-1]
		metricsBlock = 0
		for _, id := range metrics {
			metricsBlock += 1 + metricWidth(id)
		}
	}

	dimsRegion := max(innerWidth-lead-metricsBlock, len(dims)*5)
	colWidths := logTopColWidths(dims, dimsRegion)

	// Build header using renderStyledHeader so the active sort column is highlighted.
	var segments []headerSegment
	// leading spaces - no column name
	segments = append(segments, headerSegment{text: strings.Repeat(" ", lead)})
	for i, d := range dims {
		label := strings.ToUpper(d) + sortIndicatorForColumn(d)
		// Mark the dimension that the next Enter (drill-down) will expand into.
		if d != "" && d == ActiveLogTopNextDrill {
			label = logTopDrillMarker + label
		}
		raw := Truncate(label, colWidths[i])
		padded := raw + strings.Repeat(" ", max(colWidths[i]-lipgloss.Width(raw), 0))
		if i > 0 {
			segments = append(segments, headerSegment{text: " "})
		}
		segments = append(segments, headerSegment{text: padded, colName: d})
	}
	// metric columns: right-aligned within their widths, preceded by a space
	for _, name := range metrics {
		w := metricWidth(name)
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
		var metricParts strings.Builder
		for _, id := range metrics {
			metricParts.WriteByte(' ')
			metricParts.WriteString(metricCell(id, r, rowRPS))
		}
		line := strings.Repeat(" ", lead) + dimCols + metricParts.String()
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
