package app

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/ui"
)

// renderOverlayCopyField paints the ctrl+y copy picker: the visible
// table columns by default, or — after tab — every leaf field of the
// fetched manifest, each as a filterable row with the current value in
// the description column. Returns empty content + zero dims when the
// picker isn't active so the view-dispatch fallback fires.
func (m Model) renderOverlayCopyField() (string, int, int) {
	if !m.copyFieldPicker.active {
		return "", 0, 0
	}
	p := m.copyFieldPicker
	w, h, maxVisible := m.copyFieldOverlayDims()

	// RenderOverlayList pads short rows but never truncates long ones,
	// and it joins Name + "  " + Description on one line — so the two
	// share a row budget. Long paths cap at 60% of the width; the value
	// gets whatever the (possibly shorter) path leaves over. A multi-KB
	// ConfigMap value must not overflow the row and desync the
	// scrollbar column.
	innerW := max(w-4, 1)
	vis := m.visibleCopyFieldEntries()
	items := make([]ui.OverlayListItem, len(vis))
	for i, e := range vis {
		name := ui.Truncate(e.display, max(innerW*3/5, 1))
		descW := innerW - lipgloss.Width(name) - 2
		items[i] = ui.OverlayListItem{
			Name:        name,
			Description: ui.Truncate(e.value, max(descW, 0)),
		}
	}
	cfg := ui.OverlayListConfig{
		Title:           "Copy field",
		Subtitle:        copyFieldSubtitle(p),
		Cursor:          p.cursor,
		Filterable:      true,
		Filter:          p.filter,
		FilterActive:    p.filterActive,
		ShowDescription: true,
		Scroll:          p.scroll,
		MaxVisible:      maxVisible,
		EmptyMessage:    copyFieldEmptyMessage(p),
		Height:          h - 2,
	}
	return ui.RenderOverlayList(items, cfg, innerW), w, h
}

// copyFieldSubtitle names the kind, item count, and active mode with
// the tab target — "Node — columns (tab: all fields)".
func copyFieldSubtitle(p copyFieldPickerState) string {
	mode := "columns (tab: all fields)"
	if p.mode == copyFieldModeFields {
		mode = "all fields (tab: columns)"
	}
	subject := p.kind
	if p.requested > 1 {
		subject = fmt.Sprintf("%s — %d items", p.kind, p.requested)
	}
	if subject == "" {
		return mode
	}
	return subject + " — " + mode
}

// copyFieldEmptyMessage explains an empty list: fields still loading,
// fields unavailable (with the reason), or just no filter matches.
func copyFieldEmptyMessage(p copyFieldPickerState) string {
	if p.mode == copyFieldModeFields {
		if !p.fieldsLoaded {
			return "Loading fields..."
		}
		if p.fieldsErr != "" && len(p.fieldEntries) == 0 {
			return p.fieldsErr
		}
	}
	return "No matching fields"
}
