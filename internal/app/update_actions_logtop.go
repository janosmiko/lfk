package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"

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

// openLogTopFromViewer switches the open log viewer into the Log Top
// aggregation mode over the lines already buffered, without restarting the
// stream. New lines continue to flow in via the existing reader (updateLogLine
// routes to ingestLogTopLine while mode is modeLogTop).
func (m Model) openLogTopFromViewer() (tea.Model, tea.Cmd) { //nolint:unparam // tea.Cmd return is part of the action-key handler convention; may carry cmds in future
	m.mode = modeLogTop
	m.logTop = logTopState{}
	title := strings.TrimPrefix(m.logView.title, "Logs: ")
	title = strings.TrimPrefix(title, "Logs (tail): ")
	m.logTop.title = "Log Top: " + title
	m.logTopResetAndParse()
	return m, nil
}
