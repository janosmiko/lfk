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

// RenderLogTopView renders the Log Top aggregation table full screen.
// dims lists all dimension columns to show (in order).
// grouped maps dimension names that are part of the current group-by key.
// reqPerSec is the global request rate; total is the total request count used
// to compute per-row REQ/s as a proportional share of the global rate.
func RenderLogTopView(title string, dims []string, grouped map[string]bool, rows []LogTopRow, reqPerSec float64, total, cursor, scroll int, hintBar string, width, height int) string {
	titleBar := ViewTitle(width, title)

	// Lines must fit inside the bordered box: FullscreenBorderStyle renders at
	// width-2 with 1-col padding each side, leaving width-4 of text. Target
	// width-5 (matching the event viewer) so a column never wraps.
	// Layout: lead(2) + dimsRegion + metricsBlock(32)
	innerWidth := max(width-5, 20)
	const lead = 2
	const metricsBlock = 32 // " %8d %8.1f %6.1f %6d" = 1+8+1+8+1+6+1+6 = 32
	dimsRegion := max(innerWidth-lead-metricsBlock, len(dims)*5)
	colWidths := logTopColWidths(dims, dimsRegion)

	// Build header.
	headParts := make([]string, 0, len(dims))
	for i, d := range dims {
		label := strings.ToUpper(d)
		if grouped[d] {
			label = DimStyle.Bold(true).Render(label)
		} else {
			label = DimStyle.Render(label)
		}
		// Left-justify in column, accounting for ANSI.
		raw := Truncate(label, colWidths[i])
		padded := raw + strings.Repeat(" ", max(colWidths[i]-lipgloss.Width(raw), 0))
		headParts = append(headParts, padded)
	}
	dimHeader := strings.Join(headParts, " ")
	metricHeader := fmt.Sprintf(" %8s %8s %6s %6s", "REQ", "REQ/s", "%", "ERR")
	head := strings.Repeat(" ", lead) + dimHeader + metricHeader

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
