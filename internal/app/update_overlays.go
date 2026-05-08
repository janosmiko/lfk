package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Toggle: pressing the same hotkey that opened an overlay closes it.
	// When the current overlay is layered on top of another (e.g. the
	// namespace selector launched from inside RBAC), restore the parent
	// instead of dropping all the way to the explorer — and ALWAYS
	// clear previousOverlay so a stale parent can't reappear next time
	// the same hotkey opens the overlay again.
	if m.isOverlayToggleKey(msg.String()) {
		if m.previousOverlay != overlayNone {
			m.overlay = m.previousOverlay
		} else {
			m.overlay = overlayNone
		}
		m.previousOverlay = overlayNone
		return m, nil
	}
	if mdl, cmd, ok := m.handleOverlayKeyPrimary(msg); ok {
		return mdl, cmd
	}
	if mdl, cmd, ok := m.handleOverlayKeySecondary(msg); ok {
		return mdl, cmd
	}
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
	case overlayLocalClusters:
		return key == kb.LocalClusterManager
	}
	return false
}

// handleOverlayKeyPrimary dispatches overlay keys for core overlays
// (selectors, confirmations, editors).
func (m Model) handleOverlayKeyPrimary(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
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
func (m Model) handleOverlayKeySecondary(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
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
		return m.handleNetworkPolicyOverlayKey(msg), nil, true
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
	case overlayQuitConfirm:
		mdl, cmd := m.handleQuitConfirmOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogPodSelect:
		mdl, cmd := m.handleLogPodSelectOverlayKey(msg)
		return mdl, cmd, true
	case overlayLogContainerSelect:
		mdl, cmd := m.handleLogContainerSelectOverlayKey(msg)
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
		mdl, cmd := m.handleClusterColorOverlayKey(msg.String())
		return mdl, cmd, true
	case overlayLocalClusters:
		mdl, cmd, _ := m.updateLocalClusterKey(msg)
		return mdl, cmd, true
	}
	return m, nil, false
}

// handlePasteConfirmKey handles the Enter/y / Esc/n confirmation for multiline paste.
func (m Model) handlePasteConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
