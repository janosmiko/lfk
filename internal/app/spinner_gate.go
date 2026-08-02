package app

import tea "charm.land/bubbletea/v2"

// spinnerNeeded reports whether any load is in flight that the spinner
// animates: a pane/preview/metrics/helm/finalizer/command-bar load, or any
// tracked background task in the scheduler indicator.
func (m Model) spinnerNeeded() bool {
	if m.loading || m.previewLoading || m.metricsLoading ||
		m.helmRevisionsLoading || m.finalizerSearchLoading ||
		m.commandBarNameLoading != "" {
		return true
	}
	return m.scheduler != nil && m.scheduler.LenIndicator() > 0
}

// armSpinner re-arms the spinner tick loop when work is in flight and the loop
// is not already running. Called centrally from Update so no load-dispatch site
// needs to know about the spinner. The spinnerTicking guard prevents stacking.
func armSpinner(mdl tea.Model, cmd tea.Cmd) (tea.Model, tea.Cmd) {
	m, ok := mdl.(Model)
	if !ok {
		return mdl, cmd
	}
	if m.spinnerNeeded() && !m.spinnerTicking {
		m.spinnerTicking = true
		return m, tea.Batch(cmd, m.spinner.Tick)
	}
	return m, cmd
}
