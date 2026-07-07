package app

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleTaintEditorKey routes key events to the taint editor,
// delegating to the add-row sub-handler while an input field is
// focused.
func (m Model) handleTaintEditorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.taintEditor.focus != taintFocusList {
		return m.handleTaintEditorAddKey(msg)
	}
	p := &m.taintEditor
	n := len(p.rows)
	switch msg.String() {
	case "esc", "q":
		m.closeTaintEditor()
		return m, nil
	case "enter":
		return m.confirmTaintEditorApply()
	case "a":
		p.focus = taintFocusKey
		return m, nil
	case " ":
		if p.cursor < 0 || p.cursor >= n {
			return m, nil
		}
		if p.rows[p.cursor].staged {
			p.rows = append(p.rows[:p.cursor], p.rows[p.cursor+1:]...)
		} else {
			p.rows[p.cursor].remove = !p.rows[p.cursor].remove
		}
	case "j", "down":
		if p.cursor < n-1 {
			p.cursor++
		}
	case "k", "up":
		if p.cursor > 0 {
			p.cursor--
		}
	case "g", "home":
		p.cursor = 0
	case "G", "end":
		p.cursor = max(n-1, 0)
	}
	m.clampTaintEditorScroll()
	return m, nil
}

// handleTaintEditorAddKey handles typing in the add-row inputs: key,
// Tab, optional value, Tab, effect cycled with left/right. Enter
// validates and stages the taint; Esc abandons the input row.
func (m Model) handleTaintEditorAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	p := &m.taintEditor
	switch msg.String() {
	case "esc":
		p.focus = taintFocusList
		p.addKey, p.addVal, p.addEff = "", "", 0
		return m, nil
	case "tab":
		switch p.focus {
		case taintFocusKey:
			p.focus = taintFocusValue
		case taintFocusValue:
			p.focus = taintFocusEffect
		default:
			p.focus = taintFocusKey
		}
		return m, nil
	case "left", "h":
		if p.focus == taintFocusEffect {
			n := len(model.ValidTaintEffects)
			p.addEff = (p.addEff - 1 + n) % n
			return m, nil
		}
	case "right", "l":
		if p.focus == taintFocusEffect {
			p.addEff = (p.addEff + 1) % len(model.ValidTaintEffects)
			return m, nil
		}
	case "enter":
		return m.stageTaintAddition()
	case "backspace":
		switch p.focus {
		case taintFocusKey:
			p.addKey = trimLastRune(p.addKey)
		case taintFocusValue:
			p.addVal = trimLastRune(p.addVal)
		}
		return m, nil
	}
	if msg.Type == tea.KeyRunes {
		switch p.focus {
		case taintFocusKey:
			p.addKey += string(msg.Runes)
		case taintFocusValue:
			p.addVal += string(msg.Runes)
		}
	}
	return m, nil
}

// trimLastRune drops the final rune (multibyte-safe backspace).
func trimLastRune(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(r[:len(r)-1])
}

// stageTaintAddition validates the add-row inputs and appends a staged
// [+] row. Invalid input keeps the row open for correction; a
// duplicate key+effect (existing or staged) is rejected — the node
// cannot carry two taints with the same identity.
func (m Model) stageTaintAddition() (tea.Model, tea.Cmd) {
	p := &m.taintEditor
	taint := model.Taint{
		Key:    p.addKey,
		Value:  p.addVal,
		Effect: model.ValidTaintEffects[p.addEff],
	}
	if err := model.ValidateTaint(taint); err != nil {
		m.setStatusMessage("Taints: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	// Rows marked for removal don't count as duplicates — marking a
	// taint for removal and staging a replacement with the same
	// key+effect (but a new value) is the only way to edit a value,
	// and ComputeFinalTaints applies removals before additions.
	for _, r := range p.rows {
		if !r.remove && r.taint.Key == taint.Key && r.taint.Effect == taint.Effect {
			m.setStatusMessage("Taints: "+taint.Key+":"+taint.Effect+" already present", true)
			return m, scheduleStatusClear()
		}
	}
	p.rows = append(p.rows, taintEditorRow{taint: taint, staged: true})
	p.focus = taintFocusList
	p.addKey, p.addVal, p.addEff = "", "", 0
	p.cursor = len(p.rows) - 1
	m.clampTaintEditorScroll()
	return m, nil
}

// taintEditorOverlayDims returns the editor box width, height, and
// visible row count (same shape as the other overlay editors).
func (m Model) taintEditorOverlayDims() (w, h, maxVisible int) {
	w = min(m.width-6, max(m.width*70/100, 64))
	h = min(m.height-4, max(m.height*60/100, 12))
	maxVisible = max(h-9, 1) // title + subtitle + add-row(2) + footer + borders
	return w, h, maxVisible
}

// clampTaintEditorScroll keeps the cursor within the visible window via
// the shared vim-style scrolloff viewport.
func (m *Model) clampTaintEditorScroll() {
	p := &m.taintEditor
	_, _, visible := m.taintEditorOverlayDims()
	n := len(p.rows)
	p.cursor = max(0, min(p.cursor, n-1))
	overlayListScroll(&p.scroll, p.cursor, n, visible)
}

// taintEditorHints is the editor's bottom hint bar (kept here to
// co-locate the feature and keep overlay_hintbar.go under the
// file-length cap).
func (m Model) taintEditorHints() []ui.HintEntry {
	if m.taintEditor.focus != taintFocusList {
		return []ui.HintEntry{
			{Key: "tab", Desc: "next field"},
			{Key: "←/→", Desc: "effect"},
			{Key: "enter", Desc: "stage"},
			{Key: "esc", Desc: "cancel add"},
		}
	}
	return []ui.HintEntry{
		{Key: "space", Desc: "mark remove"},
		{Key: "a", Desc: "add"},
		{Key: "j/k", Desc: "navigate"},
		{Key: "enter", Desc: "apply"},
		{Key: "esc", Desc: "cancel"},
	}
}
