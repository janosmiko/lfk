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
	m.describeContent = msg.content
	// Preserve scroll/cursor on auto-refresh, reset on first load.
	if !m.describeAutoRefresh {
		m.describeScroll = 0
		m.describeCursor = 0
		m.describeCursorCol = 0
	}
	m.describeTitle = msg.title
	if m.describeAutoRefresh {
		return m, scheduleDescribeRefresh()
	}
	return m, nil
}

func (m Model) updateDescribeRefreshTick(msg describeRefreshTickMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeDescribe || !m.describeAutoRefresh || m.describeRefreshFunc == nil {
		return m, nil
	}
	return m, m.describeRefreshFunc()
}

func (m Model) updateHelmValuesLoaded(msg helmValuesLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm values: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDescribe
	m.describeContent = msg.content
	m.describeScroll = 0
	m.describeCursor = 0
	m.describeCursorCol = 0
	m.describeTitle = msg.title
	return m, nil
}

func (m Model) updateDiffLoaded(msg diffLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Diff failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDiff
	m.diffLeft = msg.left
	m.diffRight = msg.right
	m.diffLeftName = msg.leftName
	m.diffRightName = msg.rightName
	m.diffScroll = 0
	m.diffUnified = false
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
