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
	kind := m.actionCtx.kind
	isGroupResource := kind == "Deployment" || kind == "StatefulSet" || kind == "DaemonSet" ||
		kind == "Job" || kind == "CronJob" || kind == "Service"

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
	// An empty selectedContainers means "show all"; a [""] slice would filter
	// out every streamed line because no container is named "". Only pin a
	// container when one was explicitly selected.
	if m.actionCtx.containerName != "" {
		m.logView.selectedContainers = []string{m.actionCtx.containerName}
	} else {
		m.logView.selectedContainers = nil
	}
	// Group resources stream all pods via label selector; keep the parent
	// context so the log viewer (entered on Esc) can re-select pods/containers.
	if isGroupResource && m.actionCtx.containerName == "" {
		m.logView.parentKind = m.actionCtx.kind
		m.logView.parentName = m.actionCtx.name
	} else {
		m.logView.parentKind = ""
		m.logView.parentName = ""
	}
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
