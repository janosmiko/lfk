package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

// BackgroundTaskOverlayMode selects which view the overlay renders:
// in-flight tasks (default) or the completed-task history.
type BackgroundTaskOverlayMode int

const (
	// ModeRunning shows currently in-flight tasks with a live ELAPSED
	// column computed from now - StartedAt.
	ModeRunning BackgroundTaskOverlayMode = iota
	// ModeCompleted shows the recent history of finished tasks with a
	// fixed DURATION column populated by the caller from each row's
	// Duration field.
	ModeCompleted
)

// BackgroundTaskRow is the data shape consumed by the overlay renderer.
// The internal/app package converts bgtasks.Task and bgtasks.CompletedTask
// slices into this type so the renderer has zero dependencies on
// internal/app.
//
// StartedAt is only read in ModeRunning; Duration is only read in
// ModeCompleted. Callers populate whichever one matches the mode they
// pass to RenderBackgroundTasksOverlay.
type BackgroundTaskRow struct {
	Kind      string
	Name      string
	Target    string
	StartedAt time.Time
	Duration  time.Duration
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
//
// mode picks between the Running view (live ELAPSED column, "Background
// Tasks" title, "N running" footer) and the Completed view (fixed
// DURATION column, "Completed Tasks" title, "N completed" footer).
//
// scroll is the index of the first visible row. The renderer clamps it
// into [0, max) so callers can bump it blindly in response to j/k key
// presses without maintaining their own clamp logic. When the row count
// exceeds the visible window, the footer gains a "(X-Y)" position
// indicator so users know where they are in the list.
func RenderBackgroundTasksOverlay(rows []BackgroundTaskRow, mode BackgroundTaskOverlayMode, scroll, width, height int) string {
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

	title := "Background Tasks"
	emptyText := "No background tasks running."
	lastColHeader := "ELAPSED"
	footerVerb := "running"
	if mode == ModeCompleted {
		title = "Completed Tasks"
		emptyText = "No completed tasks yet."
		lastColHeader = "DURATION"
		footerVerb = "completed"
	}

	// Caller's OverlayStyle: border (1+1) + padding (2+2) = 6 cells.
	innerW := max(width-6, 20)

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	if len(rows) == 0 {
		b.WriteString(dimStyle.Render(emptyText))
		return b.String()
	}

	// Compute the visible row window. Chrome is the fixed non-data
	// content: title(1) + blank(1) + header(1) + blank-before-footer(1)
	// + footer(1) = 5 lines. The caller's OverlayStyle adds vertical
	// border+padding around the whole string; we reserve 2 more lines
	// so the rendered content never exceeds `height` under lipgloss's
	// Height() clipping.
	const chrome = 5
	const outerPad = 2
	maxVisible := max(height-chrome-outerPad, 1)
	total := len(rows)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > total-maxVisible {
		scroll = total - maxVisible
	}
	if scroll < 0 {
		scroll = 0
	}
	end := min(scroll+maxVisible, total)
	visible := rows[scroll:end]

	// Compute column widths from the data, capped to fit within innerW.
	const minKind, minName, minTarget, minLastCol = 10, 10, 10, 7
	gap := 2
	totalGaps := gap * 3

	kindW := minKind
	nameW := minName
	targetW := minTarget
	// Size columns against the visible window only. Rows scrolled off
	// screen shouldn't widen the table for content the user can't see.
	for _, r := range visible {
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
	lastColW := minLastCol
	used := kindW + nameW + targetW + lastColW + totalGaps
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
		used = kindW + nameW + targetW + lastColW + totalGaps
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
		lastColW, lastColHeader)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Data rows.
	now := time.Now()
	for _, r := range visible {
		var lastCol string
		if mode == ModeCompleted {
			lastCol = formatElapsedBGT(r.Duration)
		} else {
			lastCol = formatElapsedBGT(now.Sub(r.StartedAt))
		}
		row := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s",
			kindW, truncateBGT(r.Kind, kindW),
			nameW, truncateBGT(r.Name, nameW),
			targetW, truncateBGT(r.Target, targetW),
			lastColW, lastCol)
		b.WriteString(rowStyle.Render(row))
		b.WriteString("\n")
	}

	// Footer.
	b.WriteString("\n")
	noun := "task"
	if total != 1 {
		noun = "tasks"
	}
	footer := fmt.Sprintf("%d %s %s", total, noun, footerVerb)
	// Position indicator: only when there's content beyond the visible
	// window. "(1-15)" means rows 1 through 15 of `total` are showing.
	if total > len(visible) {
		footer += fmt.Sprintf("  (%d-%d)", scroll+1, scroll+len(visible))
	}
	b.WriteString(dimStyle.Render(footer))

	return b.String()
}

// formatElapsedBGT formats a duration for the ELAPSED/DURATION column.
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
