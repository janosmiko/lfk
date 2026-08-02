package app

import tea "charm.land/bubbletea/v2"

// executeActionCrashInvestigator handles the "Crash Investigator" action
// from the Pod action menu: kicks off the multi-section diagnostic fetch.
func (m Model) executeActionCrashInvestigator() (tea.Model, tea.Cmd) {
	m.loading = true
	m.setStatusMessage("Investigating crashes…", false)
	return m, m.loadCrashInvestigation()
}
