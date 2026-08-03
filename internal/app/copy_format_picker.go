package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// copyFormatPickerState is the open/closed/cursor model for the
// Y-key picker. scope is the selection snapshot taken at open time
// so a watch refresh between open and apply can't change which
// items are copied.
type copyFormatPickerState struct {
	active  bool
	cursor  int
	formats []CopyFormat
	scope   []model.Item
}

// openCopyFormatPicker captures the current selection (or the cursor
// row if nothing is selected) and switches the explorer into picker
// mode. No-op when there is no item to copy.
func (m *Model) openCopyFormatPicker() {
	scope := m.copyFormatPickerScope()
	if len(scope) == 0 {
		return
	}
	m.copyFormatPicker = copyFormatPickerState{
		active:  true,
		cursor:  0,
		formats: availableCopyFormats(m.nav.Level),
		scope:   scope,
	}
	m.previousOverlay = m.overlay
	m.overlay = overlayCopyFormat
}

// closeCopyFormatPicker clears picker state and restores the prior
// overlay. Idempotent.
func (m *Model) closeCopyFormatPicker() {
	m.copyFormatPicker = copyFormatPickerState{}
	if m.overlay == overlayCopyFormat {
		m.overlay = m.previousOverlay
		m.previousOverlay = overlayNone
	}
}

// copyFormatPickerStep advances the cursor by delta with wrap-around.
// Uses a two-step modulo so any integer delta (positive or negative,
// any magnitude) wraps to a valid cursor index.
func (m *Model) copyFormatPickerStep(delta int) {
	if !m.copyFormatPicker.active || len(m.copyFormatPicker.formats) == 0 {
		return
	}
	n := len(m.copyFormatPicker.formats)
	m.copyFormatPicker.cursor = ((m.copyFormatPicker.cursor+delta)%n + n) % n
}

// copyFormatPickerScope returns the items the picker will operate on:
// the visible selection if any, otherwise the cursor row. Returns nil
// when neither is available — matches handleExplorerActionKeyCopyName's
// fallthrough rules so the picker honors the same precedence today's
// Y already follows.
func (m *Model) copyFormatPickerScope() []model.Item {
	if m.hasSelection() {
		if items := m.selectedItemsList(); len(items) > 0 {
			out := make([]model.Item, len(items))
			copy(out, items)
			return out
		}
	}
	if sel := m.selectedMiddleItem(); sel != nil {
		return []model.Item{*sel}
	}
	return nil
}

// applyCopyFormatPicker resolves the picker's current cursor row to
// a tea.Cmd that produces a yamlClipboardMsg in the chosen format,
// then closes the picker. Table dispatch builds the content inline
// (no network); YAML and JSON reuse the existing dispatchYAMLClipboardCopy
// path plus wrapYAMLCmdAsJSON for JSON.
func (m Model) applyCopyFormatPicker() (tea.Model, tea.Cmd) {
	if !m.copyFormatPicker.active || m.copyFormatPicker.cursor >= len(m.copyFormatPicker.formats) {
		return m, nil
	}
	format := m.copyFormatPicker.formats[m.copyFormatPicker.cursor]
	scope := m.copyFormatPicker.scope
	m.closeCopyFormatPicker()

	// All branches operate on the captured `scope` snapshot — a watch
	// refresh between picker-open and apply cannot change what gets
	// copied. Cap-exceeded is gated upstream at picker-open time
	// (handleExplorerActionKeyCopyYAML).
	switch format {
	case CopyFormatTable:
		columns := copyTableColumnsForLevel(m.nav.Level, scope)
		content := BuildCopyTable(scope, columns)
		return m, func() tea.Msg {
			return yamlClipboardMsg{content: content, count: len(scope), format: "table"}
		}
	case CopyFormatYAML:
		cmd := m.copyYAMLForScope(scope)
		if cmd == nil {
			return m, nil
		}
		if len(scope) > 1 {
			m.setStatusMessage(fmt.Sprintf("Fetching %d manifests...", len(scope)), false)
		}
		return m, cmd
	case CopyFormatJSON:
		cmd := m.copyYAMLForScope(scope)
		if cmd == nil {
			return m, nil
		}
		if len(scope) > 1 {
			m.setStatusMessage(fmt.Sprintf("Fetching %d manifests...", len(scope)), false)
		}
		return m, wrapYAMLCmdAsJSON(cmd)
	}
	return m, nil
}

// copyTableColumnsForLevel decides which columns the Table format
// should emit for the current level. Mirrors what the explorer's
// middle column shows: Name first, then the built-ins present in
// items minus any the user hid, then the extras the user has visible
// (per ui.ActiveSessionColumns) — finally reordered to match
// ui.ActiveColumnOrder if the user reordered columns. The ui.Active*
// globals are set on every render via applySessionColumnsForKind, so
// reading them here returns the same column set the user just saw.
func copyTableColumnsForLevel(level model.Level, items []model.Item) []string {
	_ = level // reserved: a future per-level table column override (e.g., LevelClusters showing ClusterColor) would key off this param

	hiddenBuiltin := ui.ActiveHiddenBuiltinColumns
	cols := []string{"Name"}
	addBuiltinIfPresent := func(key string, accessor func(model.Item) string) {
		if hiddenBuiltin[key] {
			return
		}
		if anyNonEmpty(items, accessor) {
			cols = append(cols, key)
		}
	}
	addBuiltinIfPresent("Namespace", func(it model.Item) string { return it.Namespace })
	addBuiltinIfPresent("Ready", func(it model.Item) string { return it.Ready })
	addBuiltinIfPresent("Status", func(it model.Item) string { return it.Status })
	addBuiltinIfPresent("Restarts", func(it model.Item) string { return it.Restarts })
	addBuiltinIfPresent("Age", func(it model.Item) string { return it.Age })

	// Extras: if the user has explicitly configured a visible set via
	// the column-toggle overlay, ui.ActiveSessionColumns lists them in
	// the user-chosen order — restrict to that set. Otherwise fall back
	// to the union of Columns keys discovered in items. Either way,
	// drop the internal-prefixed keys the on-screen renderer also drops.
	var sessionSet map[string]bool
	if ui.ActiveSessionColumns != nil {
		sessionSet = make(map[string]bool, len(ui.ActiveSessionColumns))
		for _, k := range ui.ActiveSessionColumns {
			sessionSet[k] = true
		}
	}
	seen := map[string]bool{}
	for _, it := range items {
		for _, kv := range it.Columns {
			if isInternalCopyColumnKey(kv.Key) {
				continue
			}
			if sessionSet != nil && !sessionSet[kv.Key] {
				continue
			}
			if !seen[kv.Key] {
				cols = append(cols, kv.Key)
				seen[kv.Key] = true
			}
		}
	}

	// Reorder to match the user's column order if set. Name always
	// stays first; columns absent from ActiveColumnOrder fall to the
	// end in their discovery order (defensive — normally every visible
	// column is in the order list).
	if len(ui.ActiveColumnOrder) == 0 {
		return cols
	}
	present := make(map[string]bool, len(cols))
	for _, c := range cols {
		present[c] = true
	}
	ordered := []string{"Name"}
	placed := map[string]bool{"Name": true}
	for _, k := range ui.ActiveColumnOrder {
		if present[k] && !placed[k] {
			ordered = append(ordered, k)
			placed[k] = true
		}
	}
	for _, c := range cols {
		if !placed[c] {
			ordered = append(ordered, c)
			placed[c] = true
		}
	}
	return ordered
}

// isInternalCopyColumnKey reports whether an Item.Columns key is a
// renderer-only annotation that should never appear in a user-facing
// copy (matches the prefixes collectExtraToggleEntries already
// filters out for the column-toggle overlay).
func isInternalCopyColumnKey(key string) bool {
	switch {
	case strings.HasPrefix(key, "__"),
		strings.HasPrefix(key, "secret:"),
		strings.HasPrefix(key, "owner:"),
		strings.HasPrefix(key, "data:"),
		strings.HasPrefix(key, "condition:"),
		strings.HasPrefix(key, "step:"),
		strings.HasPrefix(key, "cond:"):
		return true
	}
	return false
}

// anyNonEmpty reports whether any item has a non-empty value for the
// field extracted by accessor.
func anyNonEmpty(items []model.Item, accessor func(model.Item) string) bool {
	for _, it := range items {
		if accessor(it) != "" {
			return true
		}
	}
	return false
}
