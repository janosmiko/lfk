package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateDescribeLoaded(msg describeLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Error: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeDescribe
	m.describeView.content = sanitizeDescribeContent(msg.content)
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
	m.describeView.content = sanitizeDescribeContent(msg.content)
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
	m.diffView.unified = ui.ConfigDiffViewerUnified
	return m, nil
}

// updateExplainMsg routes the API Explorer's load messages (flat level,
// recursive search, field tree) to their handlers.
func (m Model) updateExplainMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case explainLoadedMsg:
		mdl, cmd := m.updateExplainLoaded(msg)
		return mdl, cmd, true
	case explainRecursiveMsg:
		mdl, cmd := m.updateExplainRecursive(msg)
		return mdl, cmd, true
	case explainTreeLoadedMsg:
		mdl, cmd := m.updateExplainTreeLoaded(msg)
		return mdl, cmd, true
	case explainTreeDescMsg:
		return m.updateExplainTreeDescLoaded(msg), nil, true
	}
	return m, nil, false
}

func (m Model) updateExplainLoaded(msg explainLoadedMsg) (tea.Model, tea.Cmd) {
	m.loading = false
	if msg.err != nil {
		m.setErrorFromErr("Explain failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.mode = modeExplain
	m.resetExplainTree() // a fresh flat level replaces any tree visualization
	m.explainFields = msg.fields
	m.explainDesc = msg.description
	m.explainPath = msg.path
	m.explainTitle = msg.title
	m.explainCursor = 0
	m.explainScroll = 0
	m.explainSearchActive = false
	m.applyExplainPendingField()
	// Sticky tree mode: re-enter the tree for the freshly loaded level (the
	// flat list shows until the recursive result arrives).
	if m.explainTreeWanted {
		return m, m.execKubectlExplainTree(m.explainResource, m.explainAPIVersion, msg.path)
	}
	return m, nil
}

// applyExplainPendingField positions the cursor on explainPendingField (set
// when the API Explorer is opened at a specific item from the YAML viewer /
// resource tree), then clears it.
func (m *Model) applyExplainPendingField() {
	field := m.explainPendingField
	m.explainPendingField = ""
	if field == "" {
		return
	}
	for i, f := range m.explainFields {
		if f.Name == field {
			m.explainCursor = i
			m.clampExplainScroll()
			return
		}
	}
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
