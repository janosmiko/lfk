// Package app — export_template.go
// The destination picker for an exported template: clipboard, a file in the
// working directory, or the user's template directory.
package app

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

// exportDestination enumerates where a stripped manifest can go. Order is the
// picker's display order.
type exportDestination int

const (
	exportDestClipboard exportDestination = iota
	exportDestFile
	exportDestTemplateList
)

// exportDestinations is every row the picker shows.
var exportDestinations = []exportDestination{exportDestClipboard, exportDestFile, exportDestTemplateList}

func (d exportDestination) Label() string {
	switch d {
	case exportDestClipboard:
		return "Clipboard"
	case exportDestFile:
		return "File"
	case exportDestTemplateList:
		return "Template list"
	}
	return ""
}

// ShortcutKey returns the single-letter activator for a row. "c"/"f"/"t" avoid
// the j/k cursor aliases the picker also binds.
func (d exportDestination) ShortcutKey() string {
	switch d {
	case exportDestClipboard:
		return "c"
	case exportDestFile:
		return "f"
	case exportDestTemplateList:
		return "t"
	}
	return ""
}

// exportTemplateState is the open/closed/cursor model for the picker plus the
// manifest it will deliver. The manifest is captured when the picker opens, so
// a watch refresh between fetch and choice cannot change what is exported.
type exportTemplateState struct {
	active bool
	cursor int
	name   string
	kind   string
	// raw is the fetched document. Kept so a category toggle in the field
	// picker can re-strip it rather than trying to unpick the stripped copy.
	raw         string
	manifest    string
	redacted    bool
	strip       k8s.TemplateStripSet
	stripCursor int
}

// restrip recomputes the manifest after a category toggle. A strip that fails
// leaves the previous manifest in place: the picker always has something to
// deliver, and the raw document already parsed once to produce it.
func (s *exportTemplateState) restrip() {
	manifest, err := k8s.StripToTemplateWith(s.raw, s.strip)
	if err != nil {
		logger.Warn("Re-stripping the export template failed", "error", err)
		return
	}
	s.manifest = manifest
	s.redacted = k8s.TemplateRedactsValues(s.kind, s.strip)
}

// secretRedactedNote is appended to every destination's success line so the
// redaction is visible at export time, not at paste time.
const secretRedactedNote = " (Secret values redacted, keys kept)"

// redactionNote returns the suffix for the current export, empty for kinds
// whose values pass through untouched.
func (s exportTemplateState) redactionNote() string {
	if s.redacted {
		return secretRedactedNote
	}
	return ""
}

func (m *Model) openExportTemplatePicker(name, kind, raw string) {
	m.exportTemplatePicker = exportTemplateState{
		active: true,
		name:   name,
		kind:   kind,
		raw:    raw,
		strip:  loadExportStripPrefs(),
	}
	m.exportTemplatePicker.restrip()
	m.previousOverlay = overlayNone
	m.overlay = overlayExportTemplate
}

// closeExportTemplatePicker clears picker state and closes the overlay.
// Idempotent.
func (m *Model) closeExportTemplatePicker() {
	m.exportTemplatePicker = exportTemplateState{}
	if m.overlay == overlayExportTemplate {
		m.overlay = overlayNone
	}
}

// exportTemplateStep advances the cursor by delta with wrap-around.
func (m *Model) exportTemplateStep(delta int) {
	if !m.exportTemplatePicker.active {
		return
	}
	n := len(exportDestinations)
	m.exportTemplatePicker.cursor = ((m.exportTemplatePicker.cursor+delta)%n + n) % n
}

// handleExportTemplateKey routes key events to the open destination picker.
func (m Model) handleExportTemplateKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.closeExportTemplatePicker()
		return m, nil
	case "j", "down", "ctrl+n":
		m.exportTemplateStep(1)
		return m, nil
	case "k", "up", "ctrl+p":
		m.exportTemplateStep(-1)
		return m, nil
	case "enter":
		return m.applyExportTemplatePicker()
	case exportStripKey:
		m.openExportStripPicker()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	pressed := msg.String()
	for i, d := range exportDestinations {
		if pressed == d.ShortcutKey() {
			m.exportTemplatePicker.cursor = i
			return m.applyExportTemplatePicker()
		}
	}
	return m, nil
}

// applyExportTemplatePicker delivers the captured manifest to the highlighted
// destination and closes the picker.
func (m Model) applyExportTemplatePicker() (tea.Model, tea.Cmd) {
	state := m.exportTemplatePicker
	if !state.active || state.cursor >= len(exportDestinations) {
		return m, nil
	}
	dest := exportDestinations[state.cursor]
	m.closeExportTemplatePicker()

	switch dest {
	case exportDestClipboard:
		m.setStatusMessage("Copied template for "+state.name+state.redactionNote(), false)
		return m, tea.Batch(copyToSystemClipboard(state.manifest), scheduleStatusClear())
	case exportDestFile:
		path := templateExportFilename(state.kind, state.name)
		manifest, note := state.manifest, state.redactionNote()
		return m, func() tea.Msg {
			if err := writeSecureFile(path, []byte(manifest)); err != nil {
				return exportDoneMsg{err: fmt.Errorf("writing file: %w", err)}
			}
			abs, err := filepath.Abs(path)
			if err != nil {
				abs = path
			}
			return exportDoneMsg{path: abs, note: note}
		}
	case exportDestTemplateList:
		name, manifest, note := state.name, state.manifest, state.redactionNote()
		return m, func() tea.Msg {
			if err := saveUserTemplate(name, manifest); err != nil {
				return actionResultMsg{err: fmt.Errorf("saving template: %w", err)}
			}
			return actionResultMsg{message: "Saved template " + name + note}
		}
	}
	return m, nil
}

// templateExportFilename builds "<kind>_<name>.template.yaml" in the working
// directory. ".template" marks it as a stripped manifest so it is not confused
// with the "Save to file" export of the live object.
func templateExportFilename(kind, name string) string {
	safeKind := strings.ToLower(strings.ReplaceAll(kind, "/", "_"))
	safeName := strings.ReplaceAll(name, "/", "_")
	if safeKind == "" {
		return safeName + ".template.yaml"
	}
	return safeKind + "_" + safeName + ".template.yaml"
}

// exportTemplateHints is the picker's bottom hint bar. Built from the
// destination list so the chips cannot drift from the shortcut dispatch.
func exportTemplateHints() []ui.HintEntry {
	keys := make([]string, 0, len(exportDestinations))
	for _, d := range exportDestinations {
		keys = append(keys, d.ShortcutKey())
	}
	return []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: strings.Join(keys, "/"), Desc: "shortcut"},
		{Key: "enter", Desc: "export"},
		{Key: exportStripKey, Desc: "fields to remove"},
		{Key: "esc", Desc: "cancel"},
	}
}

// renderOverlayExportTemplate paints the destination picker as a small
// OverlayList. Hotkeys live on the hint bar, not in the box.
func (m Model) renderOverlayExportTemplate() (string, int, int) {
	if !m.exportTemplatePicker.active {
		return "", 0, 0
	}
	items := make([]ui.OverlayListItem, len(exportDestinations))
	for i, d := range exportDestinations {
		items[i] = ui.OverlayListItem{Key: d.ShortcutKey(), Name: d.Label()}
	}
	cfg := ui.OverlayListConfig{
		Title:        "Export template: " + ui.SanitizeTerminalText(m.exportTemplatePicker.name),
		Cursor:       m.exportTemplatePicker.cursor,
		ShowKey:      true,
		EmptyMessage: "No destinations available",
	}
	overlayW := ui.OverlayListWidth(items, cfg, max(m.width-10, 1))
	content := ui.RenderOverlayList(items, cfg, max(overlayW-4, 1))
	return content, overlayW, max(min(10, m.height-6), 1)
}
