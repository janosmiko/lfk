package app

import (
	"slices"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleTaintOverlayKey and renderTaintOverlay route the editor and its
// picker as one feature. The dispatchers they hang off (handleOverlayKeySecondary,
// renderOverlayContentExtended) sit at the gocyclo cap, so the two overlays
// share one case there and split here instead.
func (m Model) handleTaintOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.overlay == overlayTaintPresets {
		return m.handleTaintPresetKey(msg)
	}
	return m.handleTaintEditorKey(msg)
}

func (m Model) renderTaintOverlay() (string, int, int) {
	if m.overlay == overlayTaintPresets {
		return m.renderOverlayTaintPresets()
	}
	return m.renderOverlayTaintEditor()
}

// openTaintPresets opens the common-taint picker over the editor. The editor
// state stays live underneath so Esc returns to the list exactly as it was.
func (m Model) openTaintPresets() (tea.Model, tea.Cmd) {
	m.taintEditor.presetCursor = 0
	m.taintEditor.presetScroll = 0
	m.overlay = overlayTaintPresets
	return m, nil
}

// closeTaintPresets returns to the editor without adopting anything — the
// picker writes to the add-row only on an explicit selection.
func (m *Model) closeTaintPresets() {
	m.overlay = overlayTaintEditor
}

// taintPresetsMaxVisible caps the picker rows shown at once, shared by the
// renderer and the scroll clamp so the scrollbar and cursor stay in sync.
func (m Model) taintPresetsMaxVisible() int {
	return min(12, max(m.height-12, 3))
}

// handleTaintPresetKey drives the common-taint picker: vim navigation, Enter
// to adopt the highlighted preset, Esc to back out.
func (m Model) handleTaintPresetKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.taintEditor
	n := len(model.CommonTaints)
	visible := m.taintPresetsMaxVisible()
	switch msg.String() {
	case "esc", "q":
		m.closeTaintPresets()
		return m, nil
	case "enter":
		return m.applyTaintPreset()
	case "j", "down":
		if p.presetCursor < n-1 {
			p.presetCursor++
		}
	case "k", "up":
		if p.presetCursor > 0 {
			p.presetCursor--
		}
	case "g", "home":
		p.presetCursor = 0
	case "G", "end":
		p.presetCursor = max(n-1, 0)
	case "ctrl+d":
		p.presetCursor = min(p.presetCursor+visible/2, max(n-1, 0))
	case "ctrl+u":
		p.presetCursor = max(p.presetCursor-visible/2, 0)
	case "ctrl+f", "pgdown", "shift+down":
		p.presetCursor = min(p.presetCursor+visible, max(n-1, 0))
	case "ctrl+b", "pgup", "shift+up":
		p.presetCursor = max(p.presetCursor-visible, 0)
	}
	p.presetCursor = max(0, min(p.presetCursor, n-1))
	overlayListScroll(&p.presetScroll, p.presetCursor, n, visible)
	return m, nil
}

// applyTaintPreset copies the highlighted preset into the add-row and returns
// to the editor with the key field focused. The preset is a starting point,
// not a commitment: all three fields stay editable and the taint still goes
// through the normal validation and staging path.
func (m Model) applyTaintPreset() (tea.Model, tea.Cmd) {
	p := &m.taintEditor
	if p.presetCursor < 0 || p.presetCursor >= len(model.CommonTaints) {
		m.closeTaintPresets()
		return m, nil
	}
	t := model.CommonTaints[p.presetCursor].Taint
	p.addKey = t.Key
	p.addVal = t.Value
	p.addEff = max(slices.Index(model.ValidTaintEffects, t.Effect), 0)
	p.focus = taintFocusKey
	m.closeTaintPresets()
	return m, nil
}

// renderOverlayTaintPresets paints the common-taint picker: each preset in
// kubectl notation with a note on what applying it does.
func (m Model) renderOverlayTaintPresets() (string, int, int) {
	if !m.taintEditor.active {
		return "", 0, 0
	}
	p := m.taintEditor
	items := make([]ui.OverlayListItem, len(model.CommonTaints))
	for i, preset := range model.CommonTaints {
		items[i] = ui.OverlayListItem{
			Name:        preset.Taint.String(),
			Description: preset.Desc,
		}
	}
	cfg := ui.OverlayListConfig{
		Title:           "Common taints",
		Cursor:          p.presetCursor,
		ShowDescription: true,
		Scroll:          p.presetScroll,
		MaxVisible:      m.taintPresetsMaxVisible(),
	}

	overlayW := ui.OverlayListWidth(items, cfg, max(m.width-10, 1))
	innerW := max(overlayW-4, 1)
	for i := range items {
		items[i].Name = ui.Truncate(items[i].Name, innerW)
	}

	content := ui.RenderOverlayList(items, cfg, innerW)
	overlayH := min(lipgloss.Height(content)+2, max(m.height-4, 3))
	return content, overlayW, overlayH
}

// taintPresetHints is the picker's bottom hint bar.
func taintPresetHints() []ui.HintEntry {
	return []ui.HintEntry{
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "use taint"},
		{Key: "esc", Desc: "back"},
	}
}
