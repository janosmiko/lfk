// Package app — export_strip_overlay.go
// The export field-category picker. Correctness fields show as locked rows so
// the user can see what always goes without being offered a broken manifest.
package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// exportStripKey opens the field picker from the destination picker. Must not
// collide with any key handleExportTemplateKey consumes first — see
// TestExportStripKey_DoesNotCollideWithDestinationPicker.
const exportStripKey = "s"

const exportStripLockedNote = "Locked rows always go: keeping them yields a manifest that will not apply."

// exportStripRow is one line of the picker.
type exportStripRow struct {
	label string
	desc  string
}

// exportStripCategoryRows label the optional categories, in k8s.TemplateCategories order.
var exportStripCategoryRows = map[k8s.TemplateCategory]exportStripRow{
	k8s.TemplateNamespace:     {"Namespace", "metadata.namespace"},
	k8s.TemplateLabels:        {"Labels", "every author-written label"},
	k8s.TemplateAnnotations:   {"Annotations", "every author-written annotation"},
	k8s.TemplateHelmOwnership: {"Helm ownership", "helm.sh/chart, meta.helm.sh/*, heritage, release"},
	k8s.TemplateVendorRuntime: {"Vendor runtime annotations", "cni.projectcalico.org/*, field.cattle.io/*"},
	k8s.TemplateSecretValues:  {"Secret values", "keys and type are kept"},
}

// exportStripLockedRows are removed whatever the user picks.
var exportStripLockedRows = []exportStripRow{
	{"Status and server-set metadata", "uid, resourceVersion, generation, managedFields, ownerReferences"},
	{"last-applied-configuration", "kubectl's copy of the previous manifest"},
	{"Finalizers", "an object nothing in the target cluster can delete"},
	{"Controller-generated labels", "pod-template-hash, controller-uid, job-name"},
}

var overlayExportStripScrollPos int

// openExportStripPicker opens the field picker over the destination picker, so
// Esc and Ctrl+C both land back on the destinations.
func (m *Model) openExportStripPicker() {
	if !m.exportTemplatePicker.active {
		return
	}
	m.exportTemplatePicker.stripCursor = 0
	m.previousOverlay = overlayExportTemplate
	m.overlay = overlayExportStrip
}

// closeExportStripPicker returns to the destination picker.
func (m *Model) closeExportStripPicker() {
	m.previousOverlay = overlayNone
	m.overlay = overlayExportTemplate
}

// exportStripStep moves the cursor by delta, clamped to the choosable rows: the
// locked rows below them are not navigable targets.
func (m *Model) exportStripStep(delta int) {
	n := len(k8s.TemplateCategories)
	m.exportTemplatePicker.stripCursor = min(max(m.exportTemplatePicker.stripCursor+delta, 0), n-1)
}

// exportStripToggle flips the highlighted category, re-strips the captured
// document, and records the choice.
func (m *Model) exportStripToggle() {
	cursor := m.exportTemplatePicker.stripCursor
	if cursor < 0 || cursor >= len(k8s.TemplateCategories) {
		return
	}
	cat := k8s.TemplateCategories[cursor]
	m.exportTemplatePicker.strip[cat] = !m.exportTemplatePicker.strip[cat]
	m.exportTemplatePicker.restrip()
	saveExportStripPrefs(m.exportTemplatePicker.strip)
}

func (m Model) handleExportStripKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", exportStripKey:
		m.closeExportStripPicker()
	case "j", "down", "ctrl+n":
		m.exportStripStep(1)
	case "k", "up", "ctrl+p":
		m.exportStripStep(-1)
	case "ctrl+d", "shift+down":
		m.exportStripStep(len(k8s.TemplateCategories) / 2)
	case "ctrl+u", "shift+up":
		m.exportStripStep(-len(k8s.TemplateCategories) / 2)
	case "ctrl+f", "pgdown":
		m.exportStripStep(len(k8s.TemplateCategories))
	case "ctrl+b", "pgup":
		m.exportStripStep(-len(k8s.TemplateCategories))
	case "g":
		m.exportStripStep(-len(k8s.TemplateCategories))
	case "G":
		m.exportStripStep(len(k8s.TemplateCategories))
	case "space", " ", "enter", "x":
		m.exportStripToggle()
	case "r":
		m.exportTemplatePicker.strip = k8s.DefaultTemplateStripSet()
		m.exportTemplatePicker.restrip()
		saveExportStripPrefs(m.exportTemplatePicker.strip)
	}
	return m, nil
}

func exportStripHints() []ui.HintEntry {
	return []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "ctrl+d/u", Desc: "half page"},
		{Key: "space", Desc: "toggle"},
		{Key: "r", Desc: "defaults"},
		{Key: "esc", Desc: "back"},
	}
}

// exportStripItems builds the row list: the choosable categories first (their
// index is the cursor), then a divider and the locked rows.
func (m Model) exportStripItems() []ui.OverlayListItem {
	items := make([]ui.OverlayListItem, 0, len(k8s.TemplateCategories)+len(exportStripLockedRows)+1)
	for _, cat := range k8s.TemplateCategories {
		row := exportStripCategoryRows[cat]
		items = append(items, ui.OverlayListItem{
			Name:        row.label,
			Description: row.desc,
			Selected:    m.exportTemplatePicker.strip[cat],
		})
	}
	items = append(items, ui.OverlayListItem{Name: "always removed (locked)", Header: true})
	for _, row := range exportStripLockedRows {
		items = append(items, ui.OverlayListItem{
			Name:        row.label,
			Description: row.desc,
			Selected:    true,
			Disabled:    true,
		})
	}
	return items
}

// renderOverlayExportPickers renders whichever of the two export pickers is up.
func (m Model) renderOverlayExportPickers() (string, int, int, bool) {
	switch m.overlay {
	case overlayExportTemplate:
		c, w, h := m.renderOverlayExportTemplate()
		return c, w, h, true
	case overlayExportStrip:
		c, w, h := m.renderOverlayExportStrip()
		return c, w, h, true
	}
	return "", 0, 0, false
}

func (m Model) renderOverlayExportStrip() (string, int, int) {
	items := m.exportStripItems()
	// title + blank + subtitle is the chrome above the rows; sizing to it keeps
	// the box from padding out with blank lines.
	overlayH := max(min(len(items)+5, m.height-6), 1)
	contentH := max(overlayH-2, 1)
	cfg := ui.OverlayListConfig{
		Title:           "Export template: fields to remove",
		Subtitle:        exportStripLockedNote,
		Cursor:          m.exportTemplatePicker.stripCursor,
		MultiSelect:     true,
		ShowDescription: true,
		Scroll:          overlayListScroll(&overlayExportStripScrollPos, m.exportTemplatePicker.stripCursor, len(items), contentH-3),
		MaxVisible:      max(contentH-3, 1),
		EmptyMessage:    "No categories",
		Height:          contentH,
	}
	overlayW := ui.OverlayListWidth(items, cfg, max(m.width-10, 1))
	return ui.RenderOverlayList(items, cfg, max(overlayW-4, 1)), overlayW, overlayH
}
