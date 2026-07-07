package app

import (
	"strings"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayTaintEditor paints the node taint editor: existing
// taints with removal checkboxes, staged additions marked [new], and
// the add-row inputs when focused. Returns empty content + zero dims
// when the editor isn't active so the view-dispatch fallback fires.
func (m Model) renderOverlayTaintEditor() (string, int, int) {
	if !m.taintEditor.active {
		return "", 0, 0
	}
	p := m.taintEditor
	w, h, maxVisible := m.taintEditorOverlayDims()
	innerW := max(w-4, 1)

	items := make([]ui.OverlayListItem, len(p.rows))
	for i, r := range p.rows {
		it := ui.OverlayListItem{
			Name:     ui.Truncate(taintDisplayString(r.taint), innerW),
			Selected: r.remove,
		}
		if r.staged {
			it.Status = "new"
		}
		items[i] = it
	}
	cfg := ui.OverlayListConfig{
		Title:        "Taints",
		Subtitle:     p.node,
		Cursor:       p.cursor,
		MultiSelect:  true,
		ShowStatus:   true,
		Scroll:       p.scroll,
		MaxVisible:   maxVisible,
		EmptyMessage: taintEditorEmptyMessage(p),
		Height:       h - 4, // reserve two lines for the add-row block below
	}
	content := ui.RenderOverlayList(items, cfg, innerW)
	return content + "\n" + m.renderTaintEditorAddRow(innerW), w, h
}

// taintEditorEmptyMessage explains an empty list: still loading, or a
// node with no taints.
func taintEditorEmptyMessage(p taintEditorState) string {
	if p.loading {
		return "Loading taints..."
	}
	return "No taints on this node — press a to add"
}

// renderTaintEditorAddRow paints the staged-input line. Inactive: a dim
// "a to add" hint. Active: the three fields with the focused one
// highlighted and a cursor marker on the text inputs.
func (m Model) renderTaintEditorAddRow(innerW int) string {
	p := m.taintEditor
	if p.focus == taintFocusList {
		return ui.OverlayDimStyle.Render(ui.Truncate("a to add a taint", innerW))
	}
	field := func(label, val string, focused bool) string {
		text := label + " [" + val
		if focused {
			text += "▏"
		}
		text += "]"
		if focused {
			return ui.OverlayFilterStyle.Render(text)
		}
		return ui.OverlayDimStyle.Render(text)
	}
	effect := "effect <" + model.ValidTaintEffects[p.addEff] + ">"
	if p.focus == taintFocusEffect {
		effect = ui.OverlayFilterStyle.Render(effect)
	} else {
		effect = ui.OverlayDimStyle.Render(effect)
	}
	row := strings.Join([]string{
		field("add: key", p.addKey, p.focus == taintFocusKey),
		field("value", p.addVal, p.focus == taintFocusValue),
		effect,
	}, "  ")
	return ui.Truncate(row, innerW)
}

// taintDisplayString renders a taint for the overlay row with control
// bytes (hostile values could carry ANSI escapes) collapsed to spaces
// so they cannot break the layout or escape-inject the terminal.
func taintDisplayString(t model.Taint) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, t.String())
}
