// Package app — template_delete.go
// The confirmation returns to the picker either way, because the missing row
// is what tells the user the delete worked.
package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
)

// deleteTemplateAction is the pendingAction label that routes the confirmation
// back here. It is deliberately absent from mutatingActions: the file is local
// configuration, so read-only mode has no claim on it.
const deleteTemplateAction = "Delete Template"

func (m Model) beginTemplateDelete(filtered []model.ResourceTemplate) (tea.Model, tea.Cmd) {
	if m.templateCursor < 0 || m.templateCursor >= len(filtered) {
		return m, nil
	}
	tmpl := filtered[m.templateCursor]
	if tmpl.Category != userTemplateCategory {
		m.setStatusMessage("Only your own templates can be deleted", true)
		return m, scheduleStatusClear()
	}
	// The path, not the name: two files can share a base name.
	m.confirmAction = tmpl.Path
	m.pendingAction = deleteTemplateAction
	m.confirmTitle = "Confirm Delete"
	m.confirmQuestion = fmt.Sprintf("Delete the saved template %s?", tmpl.Name)
	// The cost rows belong to cluster deletes. A stale one would be shown
	// against a file that owns nothing.
	m.blast.reset()
	m.deps.reset()
	m.overlay = overlayConfirm
	return m, nil
}

func (m Model) commitTemplateDelete() (tea.Model, tea.Cmd) {
	path := m.confirmAction
	name := templateNameFromPath(path)
	m = m.clearTemplateDeleteConfirm()
	if err := deleteUserTemplate(path); err != nil {
		m.setStatusMessage("Failed to delete template "+name+": "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.templateItems = mergedTemplates()
	m.templateCursor = clampOverlayCursor(m.templateCursor, 0, len(m.filteredTemplates())-1)
	m.setStatusMessage("Deleted template "+name, false)
	return m, scheduleStatusClear()
}

// clearTemplateDeleteConfirm drops the confirmation state and reopens the
// picker.
func (m Model) clearTemplateDeleteConfirm() Model {
	m.confirmAction = ""
	m.pendingAction = ""
	m.confirmTitle = ""
	m.confirmQuestion = ""
	m.overlay = overlayTemplates
	return m
}
