package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

func (m Model) directActionForceDelete() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		return m.openBulkActionDirect("Force Delete")
	}
	kind := m.selectedResourceKind()
	if isVirtualResourceKind(kind) {
		return m, nil
	}
	sel := m.selectedMiddleItem()
	if sel == nil {
		return m, nil
	}
	m.actionCtx = m.buildActionCtx(sel, kind)
	// longhorn.io nodes are not force-deleteable by kind ("Node" collides with
	// core nodes) but support their own force-delete (disable scheduling, then
	// delete past the validating webhook).
	if !model.IsForceDeleteableKind(kind) && !model.IsLonghornNode(m.actionCtx.resourceType) {
		m.setStatusMessage("Force delete not available for "+kind, true)
		return m, scheduleStatusClear()
	}
	if msg, blocked := m.actionBlockedReason(kind, "Force Delete"); blocked {
		m.setStatusMessage(msg, true)
		return m, scheduleStatusClear()
	}
	if m.isUnionSentinel() && !isUnionAllowedActionForKind(kind, "Force Delete") {
		logger.Info("Blocked by union view", "action", "Force Delete", "kind", kind)
		m.setStatusMessage("Force Delete is not available in union view", true)
		return m, scheduleStatusClear()
	}
	m.confirmAction = sel.Name + " (FORCE)"
	m.confirmTitle = "Confirm Force Delete"
	m.confirmQuestion = fmt.Sprintf("Force delete %s?", sel.Name)
	m.confirmTypeInput.Clear()
	m.resetForceDeletePropagation()
	m.overlay = overlayConfirmType
	m.pendingAction = "Force Delete"
	m.beginDependents()
	return m, m.loadDependents()
}
