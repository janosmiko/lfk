package app

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateAutoSyncLoaded(msg autoSyncLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Loading autosync config: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.autoSyncEnabled = msg.enabled
	m.autoSyncSelfHeal = msg.selfHeal
	m.autoSyncPrune = msg.prune
	m.autoSyncCursor = 0
	m.overlay = overlayAutoSync
	return m, nil
}

func (m Model) updateAutoSyncSaved(msg autoSyncSavedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Saving autosync config: ", msg.err)
	} else {
		m.setStatusMessage("AutoSync configuration updated", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateExportDone(msg exportDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Export failed: ", msg.err)
	} else {
		m.setStatusMessage("Exported to "+msg.path, false)
	}
	return m, scheduleStatusClear()
}

func (m Model) updateRevisionList(msg revisionListMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Error loading revisions: ", msg.err)
		return m, scheduleStatusClear()
	}
	m.rollbackRevisions = msg.revisions
	m.rollbackCursor = 0
	m.overlay = overlayRollback
	return m, nil
}

func (m Model) updateRollbackDone(msg rollbackDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Rollback failed: ", msg.err)
	} else {
		m.setStatusMessage("Rollback successful", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateHelmRevisionList(msg helmRevisionListMsg) (tea.Model, tea.Cmd) {
	m.helmRevisionsLoading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm revisions: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	m.helmRollbackRevisions = msg.revisions
	m.helmRollbackCursor = 0
	m.overlay = overlayHelmRollback
	return m, nil
}

// updateHelmHistoryList handles the helmHistoryListMsg. On error the overlay
// is closed (reset to overlayNone) because executeActionHelmHistory opened it
// optimistically before the fetch completed. On success the revisions are
// populated and the overlay stays open. The shared helmRevisionsLoading flag
// is cleared in either case so the loading placeholder disappears.
func (m Model) updateHelmHistoryList(msg helmHistoryListMsg) (tea.Model, tea.Cmd) {
	m.helmRevisionsLoading = false
	if msg.err != nil {
		m.setErrorFromErr("Error loading Helm history: ", msg.err)
		m.overlay = overlayNone
		return m, scheduleStatusClear()
	}
	m.helmHistoryRevisions = msg.revisions
	m.helmHistoryCursor = 0
	m.overlay = overlayHelmHistory
	return m, nil
}

func (m Model) updateHelmRollbackDone(msg helmRollbackDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Helm rollback failed: ", msg.err)
	} else {
		m.setStatusMessage("Helm rollback successful", false)
		m.overlay = overlayNone
	}
	return m, tea.Batch(m.refreshCurrentLevel(), scheduleStatusClear())
}

func (m Model) updateTemplateApply(msg templateApplyMsg) (tea.Model, tea.Cmd) {
	if !msg.origModTime.IsZero() {
		if fi, err := os.Stat(msg.tmpFile); err == nil && fi.ModTime().Equal(msg.origModTime) {
			_ = os.Remove(msg.tmpFile)
			m.setStatusMessage("Template not saved — apply skipped", false)
			return m, scheduleStatusClear()
		}
	}
	return m, m.applyTemplateFile(msg.tmpFile, msg.context, msg.ns)
}
