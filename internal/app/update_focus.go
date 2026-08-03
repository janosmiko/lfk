package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// updateBlur handles tea.BlurMsg. It flips the focus flag so the next watch
// tick resolves to the background interval. No in-flight tick is cancelled --
// Bubble Tea has no tick-cancellation primitive and one stale fire is harmless.
func (m Model) updateBlur(_ tea.BlurMsg) (tea.Model, tea.Cmd) {
	m.focused = false
	return m, nil
}

// updateFocus handles tea.FocusMsg. Snap-back: refresh the current view so the
// user sees fresh data, reset the idle clock, and start a fresh watch chain
// (retiring any background-interval tick still in flight). Only refreshes in watch
// mode -- with watch off, the user expects manual refresh only.
func (m Model) updateFocus(_ tea.FocusMsg) (tea.Model, tea.Cmd) {
	m.focused = true
	m.lastInputAt = time.Now()
	if !m.watchThrottle || !m.watchMode {
		return m, nil
	}
	m.suppressBgtasks = true
	cmd := tea.Batch(m.refreshCurrentLevel(), m.startWatchChain())
	m.suppressBgtasks = false
	return m, cmd
}
