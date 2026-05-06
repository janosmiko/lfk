package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

// executeActionSyncWaveTimeline opens the Sync Wave Timeline overlay for
// the action-context Application using the two-phase load: opens the
// overlay IMMEDIATELY (so the user gets visual feedback within ~100ms
// instead of waiting up to 30s for the wave annotations), kicks off the
// fast skeleton fetch, and lets the message handler chain to the full
// fetch when the skeleton lands.
func (m Model) executeActionSyncWaveTimeline() (tea.Model, tea.Cmd) {
	m.syncWave.token++
	m.syncWave = syncWaveState{token: m.syncWave.token} // clear previous data + maps
	m.overlay = overlaySyncWave                         // open the overlay frame now — body shows a loading placeholder until skeleton lands
	m.loading = true
	m.setStatusMessage("Building sync wave timeline…", false)
	return m, m.loadSyncWaveTimelineSkeleton(m.syncWave.token)
}

// dispatchActionSyncWaveTimeline is the action-dispatch wrapper used by
// update_actions.go's switch. Lives here to keep update_actions.go under
// the 800-line cap.
func (m Model) dispatchActionSyncWaveTimeline() (tea.Model, tea.Cmd, bool) {
	mdl, cmd := m.executeActionSyncWaveTimeline()
	return mdl, cmd, true
}
