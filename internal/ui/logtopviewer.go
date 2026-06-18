package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// LogTopRow is one display row for the Log Top aggregation table.
type LogTopRow struct {
	Key      string
	Count    int
	ErrCount int
	Pct      float64
}

// RenderLogTopView renders the Log Top aggregation table full screen.
// reqPerSec is the global request rate; total is the total request count used
// to compute per-row REQ/s as a proportional share of the global rate.
func RenderLogTopView(title string, header []string, rows []LogTopRow, reqPerSec float64, total, cursor, scroll int, hintBar string, width, height int) string {
	titleBar := ViewTitle(width, title)

	groupLabel := strings.Join(header, "+")
	contentWidth := max(width-4, 20)
	keyWidth := max(contentWidth-32, 10) // leave room for REQ, REQ/s, %, ERR columns

	head := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s %8s %8s %6s %6s",
		keyWidth, strings.ToUpper(groupLabel), "REQ", "REQ/s", "%", "ERR"))

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
		line := fmt.Sprintf("  %-*s %8d %8.1f %6.1f %6d",
			keyWidth, Truncate(r.Key, keyWidth), r.Count, rowRPS, r.Pct, r.ErrCount)
		if i == cursor {
			line = SelectedStyle.Render(line)
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for n := end - scroll; n < maxRows; n++ {
		b.WriteByte('\n')
	}

	body := FullscreenBorderStyle(width, maxRows).Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, hintBar)
}
