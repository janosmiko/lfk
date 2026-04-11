package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// tasksOverlayScrollStep is the row delta for Ctrl+D / Ctrl+U half-page
// scrolling. It's a fixed value rather than derived from the current
// viewport because the handler doesn't know how many rows the renderer
// will be able to fit — the renderer clamps the final scroll value, so
// over-scrolling by a step or two is always harmless.
const tasksOverlayScrollStep = 5

// tasksOverlayJumpEnd is the sentinel scroll value the G key sets to
// mean "jump to the bottom". The renderer clamps any scroll value past
// the end, so this just needs to be large enough to reliably overflow
// DefaultCompletedCap (currently 50).
const tasksOverlayJumpEnd = 1_000_000

// handleBackgroundTasksOverlayKey handles keyboard input for the :tasks
// overlay. Supports esc/q to close, Tab to toggle between the
// running-tasks view and the completed-task history view, and
// j/k/ctrl+d/ctrl+u/g/G to scroll through long lists. Row navigation
// beyond scrolling and task cancellation are deliberate non-goals.
//
//nolint:unparam // consistent overlay handler signature
func (m Model) handleBackgroundTasksOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		// Reset the toggle and scroll so the next :tasks opens fresh.
		m.tasksOverlayShowCompleted = false
		m.tasksOverlayScroll = 0
		return m, nil
	case "tab":
		m.tasksOverlayShowCompleted = !m.tasksOverlayShowCompleted
		// Switching views changes the row set entirely; reset scroll
		// so the new view starts from its top.
		m.tasksOverlayScroll = 0
		return m, nil
	case "j", "down":
		m.tasksOverlayScroll++
		return m, nil
	case "k", "up":
		if m.tasksOverlayScroll > 0 {
			m.tasksOverlayScroll--
		}
		return m, nil
	case "ctrl+d":
		m.tasksOverlayScroll += tasksOverlayScrollStep
		return m, nil
	case "ctrl+u":
		m.tasksOverlayScroll -= tasksOverlayScrollStep
		if m.tasksOverlayScroll < 0 {
			m.tasksOverlayScroll = 0
		}
		return m, nil
	case "g":
		m.tasksOverlayScroll = 0
		return m, nil
	case "G":
		// Sentinel — the renderer clamps this to the real end.
		m.tasksOverlayScroll = tasksOverlayJumpEnd
		return m, nil
	}
	return m, nil
}
