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

	groupLabel := strings.ToUpper(strings.Join(header, "+"))
	// Lines must fit inside the bordered box: FullscreenBorderStyle renders at
	// width-2 with 1-col padding each side, leaving width-4 of text. Target
	// width-5 (matching the event viewer) so a column never wraps. Each row is
	// "  <key> <8 REQ> <8 REQ/s> <6 %> <6 ERR>" = 2 + keyWidth + 32 chars, so
	// keyWidth = innerWidth - 34.
	innerWidth := max(width-5, 20)
	keyWidth := max(innerWidth-34, 10)
	groupLabel = Truncate(groupLabel, keyWidth)

	head := DimStyle.Bold(true).Render(fmt.Sprintf("  %-*s %8s %8s %6s %6s",
		keyWidth, groupLabel, "REQ", "REQ/s", "%", "ERR"))

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

	// Content is the header row plus maxRows data rows; size the border to that
	// full height so the view fills the screen exactly (no blank line below the
	// hint bar) and the box doesn't grow past its budget.
	body := FullscreenBorderStyle(width, maxRows+1).Render(strings.TrimRight(b.String(), "\n"))
	return lipgloss.JoinVertical(lipgloss.Left, titleBar, body, hintBar)
}
