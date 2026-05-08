package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// tasksOverlayScrollStep is the row delta for Ctrl+D / Ctrl+U half-page
// scrolling. The handler clamps the resulting value against the live
// row count so over-scrolling by a step or two never strands the model
// past the end.
const tasksOverlayScrollStep = 5

// tasksOverlayJumpEnd is the sentinel scroll value the G key sets to
// mean "jump to the bottom". The handler clamps it down to the real
// max immediately so subsequent k presses respond on the first press.
const tasksOverlayJumpEnd = 1_000_000

// handleBackgroundTasksOverlayKey handles keyboard input for the
// :scheduler overlay. Supports esc/q to close, Tab to toggle between
// the running and completed views, and j/k/ctrl+d/ctrl+u/g/G to
// scroll through long lists. Row navigation beyond scrolling and task
// cancellation are deliberate non-goals.
//
// Every scroll-mutating branch ends with a clamp call so the model
// state can never drift past the real bounds — without this, scrolling
// down past the end leaves a stale scroll value the user has to undo
// with many up presses before the viewport actually moves.
//
//nolint:unparam // consistent overlay handler signature
func (m Model) handleBackgroundTasksOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		// Reset the toggle and scroll so the next :scheduler opens fresh.
		m.tasksOverlayShowCompleted = false
		m.tasksOverlayShowAll = false
		m.tasksOverlayScroll = 0
		m.tasksOverlayFrozenHistory = nil
		return m, nil
	case "tab":
		m.tasksOverlayShowCompleted = !m.tasksOverlayShowCompleted
		// Switching views changes the row set entirely; reset scroll
		// so the new view starts from its top.
		m.tasksOverlayScroll = 0
		m.tasksOverlayFrozenHistory = nil
		return m, nil
	case "a":
		// Toggle "show all" — only meaningful in completed mode (the
		// running view never filters anyway). Reset scroll so the
		// expanded/contracted list starts from the top, and clear the
		// frozen snapshot so the new filter takes effect immediately.
		if m.tasksOverlayShowCompleted {
			m.tasksOverlayShowAll = !m.tasksOverlayShowAll
			m.tasksOverlayScroll = 0
			m.tasksOverlayFrozenHistory = nil
		}
		return m, nil
	case "j", "down":
		m.tasksOverlayScroll = m.clampTasksOverlayScroll(m.tasksOverlayScroll + 1)
		m = m.maybeFreezeHistory()
		return m, nil
	case "k", "up":
		m.tasksOverlayScroll = m.clampTasksOverlayScroll(m.tasksOverlayScroll - 1)
		m = m.maybeFreezeHistory()
		return m, nil
	case "ctrl+d":
		m.tasksOverlayScroll = m.clampTasksOverlayScroll(m.tasksOverlayScroll + tasksOverlayScrollStep)
		m = m.maybeFreezeHistory()
		return m, nil
	case "ctrl+u":
		m.tasksOverlayScroll = m.clampTasksOverlayScroll(m.tasksOverlayScroll - tasksOverlayScrollStep)
		m = m.maybeFreezeHistory()
		return m, nil
	case "g":
		m.tasksOverlayScroll = 0
		m.tasksOverlayFrozenHistory = nil // back at top — resume live updates
		return m, nil
	case "G":
		// Jump-to-end: clamp the sentinel down to the real max
		// immediately so a follow-up k responds on the first press.
		m.tasksOverlayScroll = m.clampTasksOverlayScroll(tasksOverlayJumpEnd)
		m = m.maybeFreezeHistory()
		return m, nil
	}
	return m, nil
}

// maybeFreezeHistory captures the current completed-history rows when
// the user scrolls into the list, and clears the snapshot once they
// return to the top. While the snapshot is held, the renderer reads
// from it instead of m.scheduler.SnapshotCompleted() — so completions
// happening in the background don't shift rows under the user's
// cursor while they're trying to read the history.
func (m Model) maybeFreezeHistory() Model {
	if !m.tasksOverlayShowCompleted {
		// Running view stays live — its rows already reflect a
		// fixed-priority order and the lifecycle states are the point.
		m.tasksOverlayFrozenHistory = nil
		return m
	}
	if m.tasksOverlayScroll == 0 {
		// Back at the top: live updates resume.
		m.tasksOverlayFrozenHistory = nil
		return m
	}
	if m.tasksOverlayFrozenHistory == nil {
		m.tasksOverlayFrozenHistory = historyTasksForDisplay(
			m.scheduler.SnapshotCompleted(), m.tasksOverlayShowAll)
	}
	return m
}

// clampTasksOverlayScroll bounds a candidate scroll value into
// [0, maxScroll] where maxScroll = max(0, totalRows - viewportRows).
// totalRows reflects the rows the renderer will actually paint —
// the active table for ModeRunning (Snapshot + QueueSnapshot) and the
// grouped completed history for ModeCompleted.
func (m Model) clampTasksOverlayScroll(candidate int) int {
	if candidate < 0 {
		return 0
	}
	total := m.tasksOverlayTotalRows()
	_, h := tasksOverlaySize(m.width, m.height)
	visible := ui.VisibleRowsBackgroundTasks(h)
	maxScroll := max(total-visible, 0)
	if candidate > maxScroll {
		return maxScroll
	}
	return candidate
}

// tasksOverlayTotalRows returns the number of rows the renderer will
// produce for the current overlay mode. Mirrors what
// renderOverlayBackgroundTasks builds.
func (m Model) tasksOverlayTotalRows() int {
	if m.tasksOverlayShowCompleted {
		// historyTasksForDisplay filters out sub-second tasks (unless
		// the user has toggled showAll), so the rendered row count is
		// smaller than the raw history. While the user is scrolled
		// into the list we read from the frozen snapshot so the
		// scroll clamp matches the rendered rows exactly.
		if m.tasksOverlayFrozenHistory != nil {
			return len(m.tasksOverlayFrozenHistory)
		}
		return len(historyTasksForDisplay(m.scheduler.SnapshotCompleted(), m.tasksOverlayShowAll))
	}
	return len(buildActiveRows(m.scheduler.Snapshot(), m.scheduler.QueueSnapshot()))
}

// tasksOverlaySize returns the (width, height) the overlay renderer
// receives. Kept in lockstep with renderOverlayBackgroundTasks.
func tasksOverlaySize(modelW, modelH int) (int, int) {
	return min(120, modelW-10), min(20, modelH-6)
}
