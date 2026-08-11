package app

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Toggle: pressing the same hotkey that opened an overlay closes it.
	// Route through closeCurrentOverlay so toggle-close gets the same
	// cleanup as Esc/Ctrl+C — parent restoration, filter-state reset, and
	// dropping any pending bulk-action snapshot.
	if m.isOverlayToggleKey(msg.String()) {
		return m.closeCurrentOverlay()
	}
	// Universal Ctrl+C: close the current overlay (or restore its
	// parent) rather than fall through to closeTabOrQuit inside the
	// per-overlay handler. The tab-close behaviour lives at the
	// explorer level (handleExplorerKey) — once an overlay is open
	// Ctrl+C should mirror Esc/cancel semantics so users never
	// accidentally drop a tab while interacting with an overlay.
	if msg.String() == "ctrl+c" {
		return m.closeCurrentOverlay()
	}
	if mdl, cmd, ok := m.handleOverlayKeyPrimary(msg); ok {
		return mdl, cmd
	}
	if mdl, cmd, ok := m.handleOverlayKeySecondary(msg); ok {
		return mdl, cmd
	}
	return m, nil
}

// closeCurrentOverlay closes the active overlay, restoring its parent
// (e.g. RBAC underneath a namespace picker) when one exists. Also
// clears the per-overlay filter input + filter-mode booleans so the
// next time the same overlay opens, it starts from a clean slate
// rather than resuming whatever was typed before the user bailed.
func (m Model) closeCurrentOverlay() (tea.Model, tea.Cmd) {
	m.nsFilterMode = false
	m.canISubjectFilterMode = false
	m.templateSearchMode = false
	m.schemeFilterMode = false
	m.bookmarkSearchMode = bookmarkModeNormal
	m.logView.podFilterActive = false
	m.logView.containerFilterActive = false
	m.columnToggleFilterActive = false
	m.overlayFilter.Clear()
	m.bookmarkFilter.Clear()
	m.templateFilter.Clear()
	m.schemeFilter.Clear()
	m.logView.podFilterText = ""
	m.logView.containerFilterText = ""
	m.columnToggleFilter = ""
	// Drop any pending bulk-action snapshot — closing the overlay (Ctrl+C
	// or toggle key) abandons the action, so a later single-item action
	// must not be misrouted through the stale selection.
	m.resetBulkAction()
	// Release the blast-radius fetch for the same reason: this is the close
	// path for Ctrl+C and the toggle key, which never reach the per-overlay
	// handlers.
	m.blast.reset()
	m.deps.reset()
	if m.previousOverlay != overlayNone {
		m.overlay = m.previousOverlay
		m.previousOverlay = overlayNone
		return m, nil
	}
	m.overlay = overlayNone
	return m, nil
}

// isOverlayToggleKey returns true when key matches the hotkey that
// originally opened the current overlay. This lets users press the
// same key to close an overlay instead of reaching for Esc.
func (m Model) isOverlayToggleKey(key string) bool {
	kb := ui.ActiveKeybindings
	switch m.overlay {
	case overlayBackgroundTasks:
		return key == kb.TasksOverlay
	case overlayNamespace:
		return key == kb.NamespaceSelector
	case overlayAction:
		return key == kb.ActionMenu
	case overlayColorscheme:
		return key == kb.ThemeSelector
	case overlayFilterPreset:
		return key == kb.FilterPresets
	case overlayColumnToggle:
		return key == kb.ColumnToggle
	case overlayQuotaDashboard:
		return key == kb.QuotaDashboard
	case overlayClusterColor:
		return key == kb.ClusterColorPicker
	case overlayOrphans:
		return key == kb.OrphanOverlay
	case overlayUndeliverable:
		return key == kb.UndeliverableOverlay
	case overlaySessions:
		return key == kb.SessionManager
	case overlayLocalClusters:
		return key == kb.LocalClusterManager
	case overlayCopyFormat:
		return key == kb.CopyYAML
	case overlayCopyField:
		return key == kb.CopyField
	}
	return false
}

// handleOverlayKeyPrimary dispatches overlay keys for core overlays
// (selectors, confirmations, editors).
func (m Model) handleOverlayKeyPrimary(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch m.overlay {
	case overlayNamespace:
		mdl, cmd := m.handleNamespaceOverlayKey(msg)
		return mdl, cmd, true
	case overlayAction:
		mdl, cmd := m.handleActionOverlayKey(msg)
		return mdl, cmd, true
	case overlayConfirm:
		mdl, cmd := m.handleConfirmOverlayKey(msg)
		return mdl, cmd, true
	case overlayConfirmType:
		mdl, cmd := m.handleConfirmTypeOverlayKey(msg)
		return mdl, cmd, true
	case overlayScaleInput:
		mdl, cmd := m.handleScaleOverlayKey(msg)
		return mdl, cmd, true
	case overlayHPAScale:
		mdl, cmd := m.handleHPAScaleOverlayKey(msg)
		return mdl, cmd, true
	case overlayPVCResize:
		mdl, cmd := m.handlePVCResizeOverlayKey(msg)
		return mdl, cmd, true
	case overlayPortForward:
		mdl, cmd := m.handlePortForwardOverlayKey(msg)
		return mdl, cmd, true
	case overlayContainerSelect:
		mdl, cmd := m.handleContainerSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayPodSelect:
		mdl, cmd := m.handlePodSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayBookmarks:
		mdl, cmd := m.handleBookmarkOverlayKey(msg)
		return mdl, cmd, true
	case overlaySessions:
		mdl, cmd := m.handleSessionsOverlayKey(msg)
		return mdl, cmd, true
	case overlayTemplates:
		mdl, cmd := m.handleTemplateOverlayKey(msg)
		return mdl, cmd, true
	case overlaySecretEditor:
		mdl, cmd := m.handleSecretEditorKey(msg)
		return mdl, cmd, true
	case overlayRightsizing:
		mdl, cmd := m.handleRightsizingOverlayKey(msg)
		return mdl, cmd, true
	case overlayConfigMapEditor:
		mdl, cmd := m.handleConfigMapEditorKey(msg)
		return mdl, cmd, true
	case overlayRollback:
		mdl, cmd := m.handleRollbackOverlayKey(msg)
		return mdl, cmd, true
	case overlayHelmRollback:
		mdl, cmd := m.handleHelmRollbackOverlayKey(msg)
		return mdl, cmd, true
	case overlayHelmHistory:
		mdl, cmd := m.handleHelmHistoryOverlayKey(msg)
		return mdl, cmd, true
	case overlayLabelEditor:
		mdl, cmd := m.handleLabelEditorKey(msg)
		return mdl, cmd, true
	case overlayAutoSync:
		mdl, cmd := m.handleAutoSyncKey(msg)
		return mdl, cmd, true
	case overlayColorscheme:
		mdl, cmd := m.handleColorschemeOverlayKey(msg)
		return mdl, cmd, true
	case overlayFilterPreset:
		mdl, cmd := m.handleFilterPresetOverlayKey(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

// handleOverlayKeySecondary dispatches overlay keys for secondary overlays
// (viewers, monitoring, info panels).
func (m Model) handleOverlayKeySecondary(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch m.overlay {
	case overlayCrashInvestigator:
		mdl, cmd := m.handleCrashInvestigatorOverlayKey(msg)
		return mdl, cmd, true
	case overlaySyncWave:
		mdl, cmd := m.handleSyncWaveOverlayKey(msg)
		return mdl, cmd, true
	case overlayRBAC, overlayPodStartup:
		m.overlay = overlayNone
		return m, nil, true
	case overlayAlerts:
		mdl, cmd := m.handleAlertsOverlayKey(msg)
		return mdl, cmd, true
	case overlayBackgroundTasks:
		mdl, cmd := m.handleBackgroundTasksOverlayKey(msg)
		return mdl, cmd, true
	case overlayBatchLabel:
		mdl, cmd := m.handleBatchLabelOverlayKey(msg)
		return mdl, cmd, true
	case overlayQuotaDashboard:
		return m.handleOverlayKeyOverlayQuotaDashboard(msg), nil, true
	case overlayEventTimeline:
		mdl, cmd := m.handleEventTimelineOverlayKey(msg)
		return mdl, cmd, true
	case overlayNetworkPolicy:
		mdl, cmd := m.handleNetworkPolicyOverlayKey(msg)
		return mdl, cmd, true
	case overlayOrphans:
		mdl, cmd := m.handleOrphansKey(msg)
		return mdl, cmd, true
	case overlayCanI:
		mdl, cmd := m.handleCanIKey(msg)
		return mdl, cmd, true
	case overlayCanISubject:
		mdl, cmd := m.handleCanISubjectOverlayKey(msg)
		return mdl, cmd, true
	case overlayExplainSearch:
		mdl, cmd := m.handleExplainSearchOverlayKey(msg)
		return mdl, cmd, true
	case overlayObjectExplorerFind:
		mdl, cmd := m.handleObjectExplorerFindKey(msg)
		return mdl, cmd, true
	case overlayQuitConfirm:
		mdl, cmd := m.handleQuitConfirmOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogPodSelect:
		mdl, cmd := m.handleLogPodSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogContainerSelect:
		mdl, cmd := m.handleLogContainerSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogTopGroupBy:
		mdl, cmd := m.handleLogTopGroupByKey(msg)
		return mdl, cmd, true
	case overlayLogTopProfile:
		mdl, cmd := m.handleLogTopProfileKey(msg)
		return mdl, cmd, true
	case overlayLogTopColumns:
		mdl, cmd := m.handleLogTopColumnsKey(msg)
		return mdl, cmd, true
	case overlayFinalizerSearch:
		mdl, cmd := m.handleFinalizerSearchKey(msg)
		return mdl, cmd, true
	case overlayColumnToggle:
		mdl, cmd := m.handleColumnToggleKey(msg)
		return mdl, cmd, true
	case overlayPasteConfirm:
		mdl, cmd := m.handlePasteConfirmKey(msg)
		return mdl, cmd, true
	case overlayClusterColor:
		mdl, cmd := m.handleClusterColorOverlayKey(msg)
		return mdl, cmd, true
	case overlayLocalClusters:
		mdl, cmd, _ := m.updateLocalClusterKey(msg)
		return mdl, cmd, true
	case overlayTrafficCapture:
		mdl, cmd := m.updateOverlayCapture(msg)
		return mdl, cmd, true
	case overlayCopyFormat:
		mdl, cmd := m.handleCopyFormatPickerKey(msg)
		return mdl, cmd, true
	case overlayCopyField:
		mdl, cmd := m.handleCopyFieldPickerKey(msg)
		return mdl, cmd, true
	case overlayTaintEditor, overlayTaintPresets:
		mdl, cmd := m.handleTaintOverlayKey(msg)
		return mdl, cmd, true
	}
	return m.handleOverlayKeyTertiary(msg)
}

// handlePasteConfirmKey handles the Enter/y / Esc/n confirmation for multiline paste.
func (m Model) handlePasteConfirmKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "y", "Y":
		m.overlay = overlayNone
		if target := m.resolvePasteTarget(m.pasteTargetID); target != nil && m.pendingPaste != "" {
			flattened := strings.ReplaceAll(strings.TrimRight(m.pendingPaste, "\n"), "\n", " ")
			target.Insert(flattened)
		}
		m.pendingPaste = ""
		m.pasteTargetID = pasteTargetNone
		m.setStatusMessage("Pasted (flattened to single line)", false)
		return m, scheduleStatusClear()
	case "n", "N", "esc":
		m.overlay = overlayNone
		m.pendingPaste = ""
		m.pasteTargetID = pasteTargetNone
		m.setStatusMessage("Paste cancelled", false)
		return m, scheduleStatusClear()
	}
	return m, nil
}
