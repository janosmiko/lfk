package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// handleBackgroundTasksOverlayKey handles keyboard input for the :tasks
// overlay. v1 only supports closing the overlay; row navigation and
// cancellation are deliberate non-goals (see the spec).
//
//nolint:unparam // consistent overlay handler signature
func (m Model) handleBackgroundTasksOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		return m, nil
	}
	return m, nil
}
