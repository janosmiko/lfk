package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// executeActionLogTop handles the "Log Top" action, opening the Log Top
// aggregation viewer for the selected resource. It reuses the same log stream
// as the standard log viewer but switches to modeLogTop so the log lines are
// parsed and aggregated rather than displayed verbatim.
func (m Model) executeActionLogTop() (tea.Model, tea.Cmd) {
	m.mode = modeLogTop
	m.resetLogBuffer()
	m.logView.scroll = 0
	m.logView.follow = true
	m.logView.wrap = false
	m.logView.timestamps = true // Log Top needs timestamps for REQ/s calculation
	m.logView.hidePrefixes = !ui.ConfigLogShowPrefixes
	m.logView.previous = false
	m.logView.isMulti = false
	m.logView.multiItems = nil
	m.logView.containers = nil
	m.logView.selectedContainers = []string{m.actionCtx.containerName}
	m.logView.tailLines = ui.ConfigLogTailLines
	m.logView.hasMoreHistory = true
	m.logView.loadingHistory = false

	// Reset aggregation state; profile detection happens once lines arrive.
	m.logTop = logTopState{}
	label := resourceTitleLabel(m.actionCtx.kind, m.actionNamespace(), m.actionCtx.name)
	if m.actionCtx.containerName != "" {
		label += " [" + m.actionCtx.containerName + "]"
	}
	m.logTop.title = "Log Top: " + label

	return m, m.startLogStream()
}
