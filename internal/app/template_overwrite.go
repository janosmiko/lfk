// Package app — template_overwrite.go
// Confirms before Export Template replaces an existing saved template file.
package app

import (
	tea "charm.land/bubbletea/v2"
)

// overwriteTemplateAction is the pendingAction label that routes the
// confirmation back here. Absent from mutatingActions like
// deleteTemplateAction: the file is local configuration.
const overwriteTemplateAction = "Overwrite Template"

func (m Model) commitTemplateOverwrite() (tea.Model, tea.Cmd) {
	state := m.exportTemplatePicker
	namespace, name, manifest, note := state.namespace, state.name, state.manifest, state.redactionNote()
	m = m.clearTemplateOverwriteConfirm()
	if err := saveUserTemplate(namespace, name, manifest); err != nil {
		m.setStatusMessage("Failed to save template "+name+": "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Saved template "+name+note, false)
	return m, scheduleStatusClear()
}

func (m Model) clearTemplateOverwriteConfirm() Model {
	m.confirmAction = ""
	m.pendingAction = ""
	m.confirmTitle = ""
	m.confirmQuestion = ""
	m.closeExportTemplatePicker()
	m.overlay = overlayNone
	return m
}
