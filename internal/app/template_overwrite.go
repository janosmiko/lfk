// Package app — template_overwrite.go
// Confirms before Export Template replaces an existing saved template file.
package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
)

// overwriteTemplateAction is the pendingAction label that routes the
// confirmation back here. Absent from mutatingActions like
// deleteTemplateAction: the file is local configuration.
const overwriteTemplateAction = "Overwrite Template"

// commitTemplateOverwrite hands the write to a command rather than doing it
// here: saveUserTemplate fsyncs the file and the directory, and this runs
// inside key handling, where a slow filesystem would stall the whole UI.
func (m Model) commitTemplateOverwrite() (tea.Model, tea.Cmd) {
	state := m.exportTemplatePicker
	namespace, name, manifest, note := state.namespace, state.name, state.manifest, state.redactionNote()
	m = m.clearTemplateOverwriteConfirm()
	return m, func() tea.Msg {
		if err := saveUserTemplate(namespace, name, manifest); err != nil {
			return actionResultMsg{err: fmt.Errorf("saving template: %w", err)}
		}
		return actionResultMsg{message: "Saved template " + name + note}
	}
}

func (m Model) clearTemplateOverwriteConfirm() Model {
	m = m.clearOverwriteConfirmFields()
	m.closeExportTemplatePicker()
	m.overlay = overlayNone
	return m
}

// cancelTemplateOverwrite returns to the destination picker with the export
// still staged, so declining the overwrite leaves clipboard and file reachable
// without exporting again.
func (m Model) cancelTemplateOverwrite() Model {
	m = m.clearOverwriteConfirmFields()
	m.overlay = overlayExportTemplate
	return m
}

func (m Model) clearOverwriteConfirmFields() Model {
	m.confirmAction = ""
	m.pendingAction = ""
	m.confirmTitle = ""
	m.confirmQuestion = ""
	return m
}
