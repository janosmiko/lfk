package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// BackgroundTaskRow is the data shape consumed by the overlay renderer.
// The internal/app package converts bgtasks.Task slices into this type so
// the renderer has zero dependencies on internal/app.
type BackgroundTaskRow struct {
	Kind      string
	Name      string
	Target    string
	StartedAt time.Time
}

// RenderBackgroundTasksOverlay renders the modal content for the :tasks
// overlay. width and height are the outer overlay dimensions; the caller
// wraps this string in ui.OverlayStyle (rounded border + padding), so
// this function only emits the inner content: title, header row, data
// rows, and footer summary. No border, no padding.
//
// The caller's OverlayStyle adds 1 cell of border on each side plus 2
// cells of horizontal padding, for a total of 6 cells of horizontal
// overhead. The inner content must fit within width-6 columns or rows
// will wrap onto a second line.
func RenderBackgroundTasksOverlay(rows []BackgroundTaskRow, width, height int) string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorPrimary))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorDimmed))
	rowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorFile))
	dimStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorDimmed))

	// Caller's OverlayStyle: border (1+1) + padding (2+2) = 6 cells.
	innerW := width - 6
	if innerW < 20 {
		innerW = 20
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Background Tasks"))
	b.WriteString("\n\n")

	if len(rows) == 0 {
		b.WriteString(dimStyle.Render("No background tasks running."))
		return b.String()
	}

	// Compute column widths from the data, capped to fit within innerW.
	const minKind, minName, minTarget, minElapsed = 10, 10, 10, 7
	gap := 2
	totalGaps := gap * 3

	kindW := minKind
	nameW := minName
	targetW := minTarget
	for _, r := range rows {
		if w := len(r.Kind); w > kindW {
			kindW = w
		}
		if w := len(r.Name); w > nameW {
			nameW = w
		}
		if w := len(r.Target); w > targetW {
			targetW = w
		}
	}
	elapsedW := minElapsed
	used := kindW + nameW + targetW + elapsedW + totalGaps
	if used > innerW {
		// Shrink Name and Target proportionally.
		over := used - innerW
		shrinkName := over / 2
		shrinkTarget := over - shrinkName
		nameW -= shrinkName
		targetW -= shrinkTarget
		if nameW < minName {
			nameW = minName
		}
		if targetW < minTarget {
			targetW = minTarget
		}
		// Second pass: if Name and Target clamped and total still exceeds
		// innerW, trim kindW so the row doesn't wrap onto a second line.
		used = kindW + nameW + targetW + elapsedW + totalGaps
		if used > innerW {
			remainingOver := used - innerW
			if kindW-remainingOver < minKind {
				kindW = minKind
			} else {
				kindW -= remainingOver
			}
		}
	}

	// Header row.
	header := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s",
		kindW, "KIND",
		nameW, "NAME",
		targetW, "TARGET",
		elapsedW, "ELAPSED")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Data rows.
	now := time.Now()
	for _, r := range rows {
		row := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s",
			kindW, truncateBGT(r.Kind, kindW),
			nameW, truncateBGT(r.Name, nameW),
			targetW, truncateBGT(r.Target, targetW),
			elapsedW, formatElapsedBGT(now.Sub(r.StartedAt)))
		b.WriteString(rowStyle.Render(row))
		b.WriteString("\n")
	}

	// Footer.
	b.WriteString("\n")
	noun := "task"
	if len(rows) != 1 {
		noun = "tasks"
	}
	b.WriteString(dimStyle.Render(fmt.Sprintf("%d %s running", len(rows), noun)))

	return b.String()
}

// formatElapsedBGT formats a duration for the ELAPSED column.
//
//   - <1s   -> "0.5s"
//   - <10s  -> "3.5s"   (one decimal)
//   - <60s  -> "12s"    (whole seconds)
//   - >=60s -> "1m 30s"
func formatElapsedBGT(d time.Duration) string {
	switch {
	case d < 10*time.Second:
		// Sub-10s values render with one decimal: "0.5s", "3.5s", "9.9s".
		return fmt.Sprintf("%.1fs", d.Seconds())
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	default:
		m := int(d.Minutes())
		s := int(d.Seconds()) - m*60
		return fmt.Sprintf("%dm %ds", m, s)
	}
}

// truncateBGT shortens a string to max runes using a UTF-8-safe slice and
// an ellipsis. Matches the rune-based truncation pattern used in other
// lfk renderers.
func truncateBGT(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	if max <= 1 {
		return string(runes[:max])
	}
	return string(runes[:max-1]) + "\u2026"
}
