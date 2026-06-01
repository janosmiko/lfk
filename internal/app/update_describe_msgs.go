package app

import (
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateDescribeLoaded(msg describeLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDescribe
	m.describeView.content = msg.content
	// Preserve scroll/cursor on auto-refresh, reset on first load.
	if !m.describeView.autoRefresh {
		m.describeView.scroll = 0
		m.describeView.cursor = 0
		m.describeView.cursorCol = 0
	}
	m.describeView.title = msg.title
	if m.describeView.autoRefresh {
		return m, scheduleDescribeRefresh()
	}
	return m, nil
}

func (m Model) updateDescribeRefreshTick(msg describeRefreshTickMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeDescribe || !m.describeView.autoRefresh || m.describeView.refreshFunc == nil {
		return m, nil
	}
	return m, m.describeView.refreshFunc()
}

func (m Model) updateHelmValuesLoaded(msg helmValuesLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm values: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDescribe
	m.describeView.content = msg.content
	m.describeView.scroll = 0
	m.describeView.cursor = 0
	m.describeView.cursorCol = 0
	m.describeView.title = msg.title
	return m, nil
}

func (m Model) updateDiffLoaded(msg diffLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Diff failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDiff
	m.diffView.left = msg.left
	m.diffView.right = msg.right
	m.diffView.leftName = msg.leftName
	m.diffView.rightName = msg.rightName
	m.diffView.scroll = 0
	m.diffView.unified = false
	return m, nil
}

func (m Model) updateExplainLoaded(msg explainLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Explain failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeExplain
	m.explainFields = msg.fields
	m.explainDesc = msg.description
	m.explainPath = msg.path
	m.explainTitle = msg.title
	m.explainCursor = 0
	m.explainScroll = 0
	m.explainSearchActive = false
	return m, nil
}

func (m Model) updateExplainRecursive(msg explainRecursiveMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Recursive search failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	if len(msg.matches) == 0 {
		m.explainRecursiveResults = nil
		m.explainRecursiveCursor = 0
		m.explainRecursiveScroll = 0
		m.setStatusMessage("No fields found", true)
		return m, scheduleStatusClear()
	}
	m.explainRecursiveResults = msg.matches
	m.explainRecursiveCursor = 0
	m.explainRecursiveScroll = 0
	m.explainRecursiveFilter.Clear()
	m.explainRecursiveFilterActive = false
	m.overlay = overlayExplainSearch
	return m, nil
}
