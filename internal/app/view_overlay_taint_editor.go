package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayTaintEditor paints the node taint editor: existing
// taints with removal checkboxes, staged additions suffixed (new), and
// the add-row inputs beneath the list. The box is sized to its content
// (like the copy-as picker and action menu) — nodes carry a handful of
// taints, so a screen-proportional box would be mostly dead space.
// Returns empty content + zero dims when the editor isn't active so
// the view-dispatch fallback fires.
func (m Model) renderOverlayTaintEditor() (string, int, int) {
	if !m.taintEditor.active {
		return "", 0, 0
	}
	p := m.taintEditor
	maxVisible := m.taintEditorMaxVisible()

	// Marked-for-removal rows carry the app-wide ✓ selection mark (the
	// explorer's selectionMarker); staged additions override it with a
	// + (per-item marker overrides follow the namespace overlay's "!"
	// precedent) plus a dim (new) suffix.
	items := make([]ui.OverlayListItem, len(p.rows))
	for i, r := range p.rows {
		it := ui.OverlayListItem{
			Name:   taintDisplayString(r.taint),
			Active: r.remove,
		}
		if r.staged {
			it.Active = true
			it.ActiveMarker = "+"
			it.Description = "(new)"
		}
		items[i] = it
	}
	cfg := ui.OverlayListConfig{
		Title:            "Taints — " + p.node,
		Cursor:           p.cursor,
		ShowActiveMarker: true,
		ShowDescription:  true,
		Scroll:           p.scroll,
		MaxVisible:       maxVisible,
		EmptyMessage:     taintEditorEmptyMessage(p),
	}

	// Fit the box to the widest row (70-col floor, screen-capped), then
	// truncate names against the width that actually won. While the add-row
	// is up it joins the measurement: sizing the box from the list alone
	// pushed a long key's effect selector off the right edge.
	capW := max(m.width-10, 1)
	overlayW := ui.OverlayListWidth(items, cfg, capW)
	if p.focus != taintFocusList {
		overlayW = max(overlayW, ui.OverlayContentWidth(taintAddRowNaturalWidth(p), capW))
	}
	innerW := max(overlayW-4, 1)
	for i := range items {
		items[i].Name = ui.Truncate(items[i].Name, innerW)
	}

	content := ui.RenderOverlayList(items, cfg, innerW)
	// The add-row appears only while its inputs are focused — hotkey
	// hints (including "a to add") live on the bottom hint bar, never
	// inside the overlay box.
	if p.focus != taintFocusList {
		content += "\n\n" + m.renderTaintEditorAddRow(innerW)
	}
	// OverlayStyle's Height covers content + its 1+1 vertical padding.
	overlayH := min(lipgloss.Height(content)+2, max(m.height-4, 3))
	return content, overlayW, overlayH
}

// taintEditorMaxVisible caps the taint list rows shown at once. Shared
// by the renderer and the scroll clamp so the scrollbar and cursor
// stay in sync.
func (m Model) taintEditorMaxVisible() int {
	return min(10, max(m.height-12, 3))
}

// taintEditorEmptyMessage explains an empty list: still loading, or a
// node with no taints. The add hotkey is on the hint bar, not here.
func taintEditorEmptyMessage(p taintEditorState) string {
	if p.loading {
		return "Loading taints..."
	}
	return "No taints on this node"
}

// taintAddRowSep separates the add-row's fields when they share a line.
const taintAddRowSep = "  "

// taintAddRowFocus maps each add-row field to the focus value that owns it.
var taintAddRowFocus = []taintEditorFocus{taintFocusKey, taintFocusValue, taintFocusEffect}

// taintAddRowFields returns the add-row's three field texts, unstyled. Each
// input's value is fitted to valW visual columns (the focused one carries a
// block cursor, matching the kv-editor / secret-editor inputs); pass a
// negative valW for the untruncated natural text used to size the box.
//
// The focused value truncates from the front so the caret end — what the user
// is typing right now — stays visible, while an unfocused one keeps its
// recognizable head.
func taintAddRowFields(p taintEditorState, valW int) []string {
	input := func(label, val string, focused bool) string {
		suffix := "]"
		if focused {
			suffix = "\u2588]"
		}
		prefix := label + " ["
		if valW >= 0 {
			budget := max(valW-lipgloss.Width(prefix)-lipgloss.Width(suffix), 1)
			if focused {
				val = ui.TruncateStart(val, budget)
			} else {
				val = ui.Truncate(val, budget)
			}
		}
		return prefix + val + suffix
	}
	return []string{
		input("add: key", p.addKey, p.focus == taintFocusKey),
		input("value", p.addVal, p.focus == taintFocusValue),
		"effect <" + model.ValidTaintEffects[p.addEff] + ">",
	}
}

// taintAddRowNaturalWidth is the width the add-row wants on one line, used to
// size the overlay box before anything is truncated.
func taintAddRowNaturalWidth(p taintEditorState) int {
	fields := taintAddRowFields(p, -1)
	w := len(taintAddRowSep) * (len(fields) - 1)
	for _, f := range fields {
		w += lipgloss.Width(f)
	}
	return w
}

// renderTaintEditorAddRow paints the staged-input line: the three fields with
// the focused one highlighted and the rest dimmed. Rendered only while an
// input field is focused.
//
// The fields stack onto separate lines when they cannot share one, rather than
// the row truncating as a unit — that dropped the effect selector off the right
// edge on a long key, hiding a choice that reshapes scheduling for the whole
// node.
func (m Model) renderTaintEditorAddRow(innerW int) string {
	p := m.taintEditor
	style := func(i int) lipgloss.Style {
		if p.focus == taintAddRowFocus[i] {
			return ui.OverlayFilterStyle
		}
		return ui.OverlayDimStyle
	}

	fields := taintAddRowFields(p, -1)
	sep := taintAddRowSep
	if taintAddRowNaturalWidth(p) > innerW {
		fields = taintAddRowFields(p, innerW)
		sep = "\n"
	}
	for i, f := range fields {
		fields[i] = style(i).Render(ui.Truncate(f, innerW))
	}
	return strings.Join(fields, sep)
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
