package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/app/scheduler"
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
// The internal/app package converts scheduler.Task and scheduler.CompletedTask
// slices into this type so the renderer has zero dependencies on
// internal/app.
//
// BackgroundTaskStatus distinguishes the three lifecycle states a row
// can be in inside the active (ModeRunning) view.
type BackgroundTaskStatus int

const (
	// TaskStatusRunning is an in-flight scheduler task — Fn is executing on
	// a worker right now.
	TaskStatusRunning BackgroundTaskStatus = iota
	// TaskStatusQueued is a task that has been Submitted but is still
	// waiting for a worker. Position tells the user where it sits in
	// its priority lane.
	TaskStatusQueued
	// TaskStatusFinished is a task whose Fn has returned and which is in
	// the post-completion linger window. The same task is also in the
	// Completed history; it leaves the active view once the linger
	// window expires.
	TaskStatusFinished
)

// BackgroundTaskRow is the data shape consumed by the overlay renderer.
// The internal/app package converts scheduler.Task, scheduler.QueueEntry,
// and scheduler.CompletedTask slices into one unified slice so the
// renderer has zero dependencies on internal/app and can produce a
// single table with a STATUS column.
//
// Field semantics by Status:
//   - TaskStatusRunning: StartedAt is set; Duration/FinishedAt/Position are zero.
//     ELAPSED ticks from now-StartedAt.
//   - TaskStatusQueued: StartedAt is zero; Position is the 1-based head-of-lane
//     position. ELAPSED renders as a placeholder.
//   - TaskStatusFinished: StartedAt and FinishedAt are set; Duration is the
//     completion duration. ELAPSED freezes at Duration.
//   - ModeCompleted history rows: Duration is set; everything else is
//     informational. The renderer reads only Duration.
type BackgroundTaskRow struct {
	Status     BackgroundTaskStatus
	Kind       string
	Priority   scheduler.Priority
	Name       string
	Target     string
	StartedAt  time.Time
	FinishedAt time.Time     // zero while running; set when in linger window
	Duration   time.Duration // (FinishedAt - StartedAt) for finished rows
	Position   int           // 1-based queue position; only for TaskStatusQueued
}

// RenderBackgroundTasksOverlay renders the modal content for the :scheduler
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
// mode picks between the Running view (live ELAPSED column, "Scheduler
// — Running" title; rows show STATUS = Running / Queued #N / Finished)
// and the Completed view (fixed DURATION column, "Scheduler — Completed"
// title, "N completed" footer).
//
// rows is the unified active-table slice — callers merge running,
// queued, and finished-lingering tasks into one ordered list and tag
// each with Status. Eliminating the separate Queued section keeps the
// overlay box at a stable height as items flow through the lifecycle.
//
// scroll is the index of the first visible row. The renderer clamps it
// into [0, max) so callers can bump it blindly in response to j/k key
// presses without maintaining their own clamp logic. When the row count
// exceeds the visible window, the footer gains a "(X-Y)" position
// indicator so users know where they are in the list.
//
// The data area is padded to a fixed line count so the overlay box
// stays a constant size regardless of how many rows are present —
// queued items appearing/draining no longer make the window jump.
func RenderBackgroundTasksOverlay(rows []BackgroundTaskRow, mode BackgroundTaskOverlayMode, scroll, width, height int) string {
	return RenderBackgroundTasksOverlayWithSubtitle(rows, mode, "", scroll, width, height)
}

// RenderBackgroundTasksOverlayWithSubtitle renders the overlay with an
// optional dim suffix appended to the title — used to surface state
// like "(showing all entries)" when the user has toggled the
// sub-second filter off via `a`. The empty-subtitle variant matches
// RenderBackgroundTasksOverlay exactly.
func RenderBackgroundTasksOverlayWithSubtitle(rows []BackgroundTaskRow, mode BackgroundTaskOverlayMode, subtitle string, scroll, width, height int) string {
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

	title := "Scheduler — Running"
	emptyText := "No tasks running."
	lastColHeader := "ELAPSED"
	footerVerb := "active"
	if mode == ModeCompleted {
		title = "Scheduler — Completed"
		emptyText = "No completed tasks yet."
		lastColHeader = "DURATION"
		footerVerb = "completed"
	}

	// Caller's OverlayStyle: border (1+1) + padding (2+2) = 6 cells.
	innerW := max(width-6, 20)
	dataAreaH := VisibleRowsBackgroundTasks(height)

	statusW, prioW, kindW, nameW, targetW := bgtColumnWidthsUnified(rows, innerW)
	lastColW := bgtLastColW

	total := len(rows)
	if scroll < 0 {
		scroll = 0
	}
	if scroll > total-dataAreaH {
		scroll = total - dataAreaH
	}
	if scroll < 0 {
		scroll = 0
	}
	end := min(scroll+dataAreaH, total)

	var visible []BackgroundTaskRow
	if total > 0 {
		visible = rows[scroll:end]
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(title))
	if subtitle != "" {
		b.WriteString("  ")
		b.WriteString(dimStyle.Render(subtitle))
	}
	b.WriteString("\n\n")

	// Header row.
	header := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s  %-*s",
		statusW, "STATUS",
		prioW, "PRIORITY",
		kindW, "KIND",
		nameW, "NAME",
		targetW, "TARGET",
		lastColW, lastColHeader)
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Data rows — pad to dataAreaH so the box stays at a constant size.
	now := time.Now()
	rendered := 0
	for _, r := range visible {
		statusText, statusStyle := bgtStatusCell(r, statusW, mode, dimStyle)
		lastCol := bgtLastColCell(r, mode, now)
		body := fmt.Sprintf("%-*s  %-*s  %-*s  %-*s  %-*s",
			prioW, truncateBGT(priorityLabel(r.Priority), prioW),
			kindW, truncateBGT(r.Kind, kindW),
			nameW, truncateBGT(r.Name, nameW),
			targetW, truncateBGT(r.Target, targetW),
			lastColW, lastCol)
		// Queued and finished rows render dimmer than running rows so
		// the user's eye lands on what's actively executing.
		bodyRendered := rowStyle.Render(body)
		if r.Status == TaskStatusQueued || r.Status == TaskStatusFinished {
			bodyRendered = dimStyle.Render(body)
		}
		b.WriteString(statusStyle.Render(statusText))
		b.WriteString("  ")
		b.WriteString(bodyRendered)
		b.WriteString("\n")
		rendered++
	}
	// Empty-state message centred in the data area when there's nothing
	// to show. Placed on the first data row so the rest of the area is
	// padded normally.
	if rendered == 0 {
		b.WriteString(dimStyle.Render(emptyText))
		b.WriteString("\n")
		rendered++
	}
	// Pad remaining rows with blank lines so the overlay box height is
	// stable regardless of row count — this is the fix for the
	// "window grows and shrinks as queued items appear/drain" jank.
	for rendered < dataAreaH {
		b.WriteString("\n")
		rendered++
	}

	// Footer.
	b.WriteString("\n")
	footer := bgtFooter(rows, mode, footerVerb, scroll, len(visible))
	b.WriteString(dimStyle.Render(footer))

	return b.String()
}

// bgtFooter builds the summary line. In ModeRunning it counts each
// status separately so the user sees "3 running, 2 queued, 1 finished"
// at a glance.
func bgtFooter(rows []BackgroundTaskRow, mode BackgroundTaskOverlayMode, completedVerb string, scroll, visibleCount int) string {
	total := len(rows)
	if mode == ModeCompleted {
		noun := "task"
		if total != 1 {
			noun = "tasks"
		}
		footer := fmt.Sprintf("%d %s %s", total, noun, completedVerb)
		if total > visibleCount {
			footer += fmt.Sprintf("  (%d-%d)", scroll+1, scroll+visibleCount)
		}
		return footer
	}
	var running, queued, finished int
	for _, r := range rows {
		switch r.Status {
		case TaskStatusQueued:
			queued++
		case TaskStatusFinished:
			finished++
		default:
			running++
		}
	}
	footer := fmt.Sprintf("%d running, %d queued, %d finished", running, queued, finished)
	if total > visibleCount {
		footer += fmt.Sprintf("  (%d-%d)", scroll+1, scroll+visibleCount)
	}
	return footer
}

// bgtStatusCell returns the STATUS column text and the style to render
// it with. Queued shows the position; Finished gets a dim chip; Running
// keeps the primary colour so the eye lands on it first.
func bgtStatusCell(r BackgroundTaskRow, statusW int, mode BackgroundTaskOverlayMode, dim lipgloss.Style) (string, lipgloss.Style) {
	if mode == ModeCompleted {
		return fmt.Sprintf("%-*s", statusW, "Done"), dim
	}
	switch r.Status {
	case TaskStatusQueued:
		txt := fmt.Sprintf("Queued #%d", r.Position)
		return fmt.Sprintf("%-*s", statusW, truncateBGT(txt, statusW)), dim
	case TaskStatusFinished:
		return fmt.Sprintf("%-*s", statusW, "Finished"), dim
	default:
		st := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorSecondary))
		return fmt.Sprintf("%-*s", statusW, "Running"), st
	}
}

// bgtLastColCell formats the ELAPSED/DURATION cell for one row. Queued
// rows show a placeholder since they haven't started.
func bgtLastColCell(r BackgroundTaskRow, mode BackgroundTaskOverlayMode, now time.Time) string {
	switch {
	case mode == ModeCompleted:
		return formatElapsedBGT(r.Duration)
	case r.Status == TaskStatusQueued:
		return "—"
	case r.Status == TaskStatusFinished:
		return formatElapsedBGT(r.Duration)
	default:
		return formatElapsedBGT(now.Sub(r.StartedAt))
	}
}

// priorityLabel returns the short uppercase label for a Priority value,
// matching the chip text painted in the :scheduler overlay.
func priorityLabel(p scheduler.Priority) string {
	switch p {
	case scheduler.PriorityCritical:
		return "CRITICAL"
	case scheduler.PriorityHigh:
		return "HIGH"
	case scheduler.PriorityLow:
		return "LOW"
	default:
		return ""
	}
}

// bgtLastColW is the fixed width of the ELAPSED/DURATION column.
const bgtLastColW = 6

// bgtChromeRows is the line count consumed by non-data content in the
// overlay (title + blank + header + blank-before-footer + footer).
const bgtChromeRows = 5

// bgtOuterPadRows reserves rows for the OverlayStyle border + padding
// added by the caller around the rendered string.
const bgtOuterPadRows = 2

// VisibleRowsBackgroundTasks returns the data-area height (max visible
// rows) for the background-tasks overlay given the outer overlay
// height. Exported so the keyboard handler can clamp scroll values
// against the same viewport the renderer uses; otherwise scroll-down
// past the end leaves a stale scroll value the user has to press
// scroll-up many times to undo.
func VisibleRowsBackgroundTasks(height int) int {
	return max(height-bgtChromeRows-bgtOuterPadRows, 1)
}

// bgtColumnWidthsUnified computes column widths for the unified table.
// Returns (statusW, prioW, kindW, nameW, targetW). lastColW is fixed at
// bgtLastColW.
func bgtColumnWidthsUnified(rows []BackgroundTaskRow, innerW int) (int, int, int, int, int) {
	const minStatus, minPrio, minKind, minName, minTarget = 8, 4, 6, 6, 6
	const totalGaps = 10 // 5 gaps * 2

	// "Queued #999" is 11 chars; cap at 11 to bound the column. Most
	// rows show "Running" (7) or "Finished" (8) or "Queued #N" (9-11).
	statusW := minStatus
	prioW := len("PRIORITY")
	kindW := minKind
	nameW := minName
	targetW := minTarget

	for _, r := range rows {
		if r.Status == TaskStatusQueued {
			// "Queued #N" — at least 9 chars even for single-digit positions.
			candidate := 9
			if r.Position >= 10 {
				candidate = 10
			}
			if r.Position >= 100 {
				candidate = 11
			}
			if candidate > statusW {
				statusW = candidate
			}
		}
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

	used := statusW + prioW + kindW + nameW + targetW + bgtLastColW + totalGaps
	if used <= innerW {
		// Expand Name and Target to absorb remaining inner width so the
		// table fills the whole overlay. Target gets the wider half
		// because cluster + namespace identifiers are typically longer.
		spare := innerW - used
		grow := spare / 2
		targetW += spare - grow
		nameW += grow
		return statusW, prioW, kindW, nameW, targetW
	}

	// Shrink Name and Target proportionally.
	over := used - innerW
	shrinkName := over / 2
	shrinkTarget := over - shrinkName
	nameW = max(nameW-shrinkName, minName)
	targetW = max(targetW-shrinkTarget, minTarget)

	// Second pass: trim kindW.
	used = statusW + prioW + kindW + nameW + targetW + bgtLastColW + totalGaps
	if used > innerW {
		kindW = max(kindW-(used-innerW), minKind)
	}

	// Third pass: trim prioW.
	used = statusW + prioW + kindW + nameW + targetW + bgtLastColW + totalGaps
	if used > innerW {
		prioW = max(prioW-(used-innerW), minPrio)
	}

	// Last resort: trim statusW.
	used = statusW + prioW + kindW + nameW + targetW + bgtLastColW + totalGaps
	if used > innerW {
		statusW = max(statusW-(used-innerW), minStatus)
	}

	return statusW, prioW, kindW, nameW, targetW
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
