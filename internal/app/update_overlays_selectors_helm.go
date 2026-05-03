package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleRollbackOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.rollbackRevisions = nil
		return m, nil
	case "j", "down":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, 1, len(m.rollbackRevisions)-1)
		return m, nil
	case "k", "up":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, -1, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+d":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, 10, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+u":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, -10, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+f", "pgdown":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, 20, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+b", "pgup":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, -20, len(m.rollbackRevisions)-1)
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.rollbackCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G", "end":
		if len(m.rollbackRevisions) > 0 {
			m.rollbackCursor = len(m.rollbackRevisions) - 1
		}
		return m, nil
	case "home":
		m.pendingG = false
		m.rollbackCursor = 0
		return m, nil
	case "enter":
		if m.rollbackCursor >= 0 && m.rollbackCursor < len(m.rollbackRevisions) {
			rev := m.rollbackRevisions[m.rollbackCursor]
			m.addLogEntry("DBG", fmt.Sprintf("Rolling back to revision %d (RS: %s)", rev.Revision, rev.Name))
			m.loading = true
			return m, m.rollbackDeployment(rev.Revision)
		}
		return m, nil
	case "y":
		return m.handleRollbackOverlayCopy()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleRollbackOverlayCopy() (tea.Model, tea.Cmd) {
	if m.rollbackCursor < 0 || m.rollbackCursor >= len(m.rollbackRevisions) {
		return m, nil
	}
	rev := m.rollbackRevisions[m.rollbackCursor]
	row := joinTSV(
		fmt.Sprintf("%d", rev.Revision),
		rev.Name,
		fmt.Sprintf("%d", rev.Replicas),
		strings.Join(rev.Images, ","),
		ui.FormatAge(rev.CreatedAt),
	)
	m.setStatusMessage(fmt.Sprintf("Copied revision %d", rev.Revision), false)
	return m, tea.Batch(copyToSystemClipboard(row), scheduleStatusClear())
}

func joinTSV(cols ...string) string {
	return strings.Join(cols, "\t")
}

func (m Model) handleHelmRollbackOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.helmRollbackRevisions = nil
		return m, nil
	case "j", "down":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, 1, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "k", "up":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, -1, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+d":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, 10, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+u":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, -10, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+f", "pgdown":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, 20, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+b", "pgup":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, -20, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.helmRollbackCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G", "end":
		if len(m.helmRollbackRevisions) > 0 {
			m.helmRollbackCursor = len(m.helmRollbackRevisions) - 1
		}
		return m, nil
	case "home":
		m.pendingG = false
		m.helmRollbackCursor = 0
		return m, nil
	case "enter":
		if m.helmRollbackCursor >= 0 && m.helmRollbackCursor < len(m.helmRollbackRevisions) {
			rev := m.helmRollbackRevisions[m.helmRollbackCursor]
			m.addLogEntry("DBG", fmt.Sprintf("Rolling back Helm release to revision %d", rev.Revision))
			m.loading = true
			return m, m.rollbackHelmRelease(rev.Revision)
		}
		return m, nil
	case "y":
		return m.handleHelmRollbackOverlayCopy()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleHelmRollbackOverlayCopy() (tea.Model, tea.Cmd) {
	if m.helmRollbackCursor < 0 || m.helmRollbackCursor >= len(m.helmRollbackRevisions) {
		return m, nil
	}
	rev := m.helmRollbackRevisions[m.helmRollbackCursor]
	row := joinTSV(
		fmt.Sprintf("%d", rev.Revision),
		rev.Status,
		rev.Chart,
		rev.AppVersion,
		rev.Description,
		rev.Updated,
	)
	m.setStatusMessage(fmt.Sprintf("Copied revision %d", rev.Revision), false)
	return m, tea.Batch(copyToSystemClipboard(row), scheduleStatusClear())
}

func (m Model) handleHelmHistoryOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.helmHistoryRevisions = nil
		return m, nil
	case "j", "down":
		m.helmHistoryCursor = clampOverlayCursor(m.helmHistoryCursor, 1, len(m.helmHistoryRevisions)-1)
		return m, nil
	case "k", "up":
		m.helmHistoryCursor = clampOverlayCursor(m.helmHistoryCursor, -1, len(m.helmHistoryRevisions)-1)
		return m, nil
	case "ctrl+d":
		m.helmHistoryCursor = clampOverlayCursor(m.helmHistoryCursor, 10, len(m.helmHistoryRevisions)-1)
		return m, nil
	case "ctrl+u":
		m.helmHistoryCursor = clampOverlayCursor(m.helmHistoryCursor, -10, len(m.helmHistoryRevisions)-1)
		return m, nil
	case "ctrl+f", "pgdown":
		m.helmHistoryCursor = clampOverlayCursor(m.helmHistoryCursor, 20, len(m.helmHistoryRevisions)-1)
		return m, nil
	case "ctrl+b", "pgup":
		m.helmHistoryCursor = clampOverlayCursor(m.helmHistoryCursor, -20, len(m.helmHistoryRevisions)-1)
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.helmHistoryCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G", "end":
		if len(m.helmHistoryRevisions) > 0 {
			m.helmHistoryCursor = len(m.helmHistoryRevisions) - 1
		}
		return m, nil
	case "home":
		m.pendingG = false
		m.helmHistoryCursor = 0
		return m, nil
	case "y":
		return m.handleHelmHistoryOverlayCopy()
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleHelmHistoryOverlayCopy() (tea.Model, tea.Cmd) {
	if m.helmHistoryCursor < 0 || m.helmHistoryCursor >= len(m.helmHistoryRevisions) {
		return m, nil
	}
	rev := m.helmHistoryRevisions[m.helmHistoryCursor]
	row := joinTSV(
		fmt.Sprintf("%d", rev.Revision),
		rev.Status,
		rev.Chart,
		rev.AppVersion,
		rev.Description,
		rev.Updated,
	)
	m.setStatusMessage(fmt.Sprintf("Copied revision %d", rev.Revision), false)
	return m, tea.Batch(copyToSystemClipboard(row), scheduleStatusClear())
}
