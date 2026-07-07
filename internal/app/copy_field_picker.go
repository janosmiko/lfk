package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// copyFieldMode selects which entry list the ctrl+y picker shows.
type copyFieldMode int

const (
	copyFieldModeColumns copyFieldMode = iota // visible table columns (instant, no fetch)
	copyFieldModeFields                       // every manifest leaf field (needs the fetch)
)

// copyFieldMemory records the last-copied entry for a kind so the next
// ctrl+y preselects it (session-only).
type copyFieldMemory struct {
	mode    copyFieldMode
	display string
}

// copyFieldPickerState is the ctrl+y field picker. It opens instantly
// in columns mode on the scope snapshot; the manifest fetch runs in the
// background and fills fieldEntries/docs for the tab-toggled fields
// mode. entries are flattened from docs[0] only — the first item
// defines the field list, the rest just contribute values.
type copyFieldPickerState struct {
	active       bool
	mode         copyFieldMode
	cursor       int
	scroll       int
	filter       string
	filterActive bool

	columnEntries []copyFieldEntry
	fieldEntries  []copyFieldEntry
	visible       []copyFieldEntry // current mode's entries matching filter (recomputeCopyFieldVisible)

	fieldsLoaded bool   // manifest fetch finished (successfully or not)
	fieldsErr    string // why fields mode is empty ("" when fine)

	docs      []any        // parsed manifests, fetch order
	scope     []model.Item // selection snapshot for column extraction
	kind      string       // memory key
	requested int          // scope size, for status messaging
	seq       int          // fetch sequence — stale copyFieldManifestsMsg are dropped
}

// handleExplorerActionKeyCopyField opens the ctrl+y picker immediately
// in columns mode on the scope snapshot (same precedence as Y) and
// kicks off the manifest fetch in the background for the tab-toggled
// fields mode. Over-cap or level-unsupported fetches degrade to
// columns-only with the reason surfaced when the user presses tab.
func (m Model) handleExplorerActionKeyCopyField() (tea.Model, tea.Cmd, bool) {
	scope := m.copyFormatPickerScope()
	if len(scope) == 0 {
		return m, nil, true
	}
	kind := scope[0].Kind
	if kind == "" {
		kind = m.nav.ResourceType.Kind
	}

	var fetchCmd tea.Cmd
	fieldsErr := ""
	if len(scope) > maxBulkYAMLCopy {
		fieldsErr = fmt.Sprintf("Max %d items for field copy", maxBulkYAMLCopy)
	} else if fetchCmd = m.copyYAMLForScope(scope); fetchCmd == nil {
		fieldsErr = "Fields are not available at this level"
	}

	seq := m.copyFieldPicker.seq + 1
	m.copyFieldPicker = copyFieldPickerState{
		active:        true,
		mode:          copyFieldModeColumns,
		columnEntries: buildCopyFieldColumnEntries(m.nav.Level, scope),
		fieldsLoaded:  fetchCmd == nil,
		fieldsErr:     fieldsErr,
		scope:         scope,
		kind:          kind,
		requested:     len(scope),
		seq:           seq,
	}
	m.recomputeCopyFieldVisible()
	m.copyFieldPickerPreselect()
	m.previousOverlay = m.overlay
	m.overlay = overlayCopyField
	if fetchCmd == nil {
		return m, nil, true
	}
	return m, wrapYAMLCmdForCopyField(fetchCmd, len(scope), seq), true
}

// buildCopyFieldColumnEntries lists the visible table columns of the
// scope as picker rows: the column header as the path, the first
// item's cell as the value. Reuses the Y-table column set so the rows
// match what the explorer displays.
func buildCopyFieldColumnEntries(level model.Level, scope []model.Item) []copyFieldEntry {
	cols := copyTableColumnsForLevel(level, scope)
	entries := make([]copyFieldEntry, 0, len(cols))
	for _, c := range cols {
		entries = append(entries, copyFieldEntry{
			column:  c,
			display: ui.ColumnHeaderLabel(c),
			value:   copyFieldDisplayValue(copyTableCellValue(&scope[0], c)),
		})
	}
	return entries
}

// copyFieldManifestsMsg carries the parsed manifests for the field
// picker's fields mode. err is non-nil when the underlying bulk fetch
// failed (entirely or partially — docs holds whatever did parse).
type copyFieldManifestsMsg struct {
	docs      []any
	requested int
	seq       int
	err       error
}

// wrapYAMLCmdForCopyField converts the yamlClipboardMsg produced by a
// copyYAMLForScope cmd into a copyFieldManifestsMsg, parsing the
// multi-doc payload off the UI loop. Non-yamlClipboardMsg results
// (e.g. a coalesced fetch's nil) pass through unchanged.
func wrapYAMLCmdForCopyField(cmd tea.Cmd, requested, seq int) tea.Cmd {
	return func() tea.Msg {
		msg := cmd()
		yc, ok := msg.(yamlClipboardMsg)
		if !ok {
			return msg
		}
		return copyFieldManifestsMsg{
			docs:      parseManifestDocs(yc.content, requested),
			requested: requested,
			seq:       seq,
			err:       yc.err,
		}
	}
}

// updateCopyFieldManifests fills the picker's fields mode from a
// completed background fetch. Messages for a closed picker or a stale
// fetch (the user closed and reopened meanwhile) are dropped.
func (m Model) updateCopyFieldManifests(msg copyFieldManifestsMsg) (tea.Model, tea.Cmd) {
	p := &m.copyFieldPicker
	if !p.active || msg.seq != p.seq {
		return m, nil
	}
	p.fieldsLoaded = true
	if len(msg.docs) == 0 {
		reason := "no manifests fetched"
		if msg.err != nil {
			reason = msg.err.Error()
		}
		p.fieldsErr = reason
		return m, nil
	}
	p.docs = msg.docs
	p.fieldEntries = flattenCopyFields(msg.docs[0])
	if len(p.fieldEntries) == 0 {
		p.fieldsErr = "No fields found"
	}
	if p.kind == "" {
		p.kind = manifestKind(msg.docs[0])
	}
	if p.mode == copyFieldModeFields {
		m.recomputeCopyFieldVisible()
		m.copyFieldPickerPreselect()
	}
	if msg.err != nil {
		m.setStatusMessage("Copy field: "+msg.err.Error(), true)
		return m, scheduleStatusClear()
	}
	return m, nil
}

// toggleCopyFieldMode flips columns <-> fields, resetting filter and
// cursor — the two lists share nothing, so a carried-over filter would
// just confuse.
func (m *Model) toggleCopyFieldMode() {
	p := &m.copyFieldPicker
	if p.mode == copyFieldModeColumns {
		p.mode = copyFieldModeFields
	} else {
		p.mode = copyFieldModeColumns
	}
	p.filter = ""
	p.filterActive = false
	p.cursor = 0
	p.scroll = 0
	m.recomputeCopyFieldVisible()
	m.copyFieldPickerPreselect()
}

// manifestKind returns the manifest's kind field, or "" when absent.
func manifestKind(doc any) string {
	obj, ok := doc.(map[string]any)
	if !ok {
		return ""
	}
	kind, _ := obj["kind"].(string)
	return kind
}

// copyFieldPickerCurrentEntries returns the active mode's full list.
func (m Model) copyFieldPickerCurrentEntries() []copyFieldEntry {
	if m.copyFieldPicker.mode == copyFieldModeColumns {
		return m.copyFieldPicker.columnEntries
	}
	return m.copyFieldPicker.fieldEntries
}

// copyFieldPickerPreselect moves the cursor onto the entry last copied
// for this kind (session memory) when it belongs to the active mode.
// No-op otherwise.
func (m *Model) copyFieldPickerPreselect() {
	remembered, ok := m.lastCopyFieldByKind[m.copyFieldPicker.kind]
	if !ok || remembered.mode != m.copyFieldPicker.mode {
		return
	}
	for i, e := range m.copyFieldPicker.visible {
		if e.display == remembered.display {
			m.copyFieldPicker.cursor = i
			m.clampCopyFieldScroll()
			return
		}
	}
}

// closeCopyFieldPicker clears picker state and restores the prior
// overlay. Idempotent. The fetch sequence counter survives the reset
// so an in-flight fetch from this picker can never be mistaken for the
// next one's.
func (m *Model) closeCopyFieldPicker() {
	m.copyFieldPicker = copyFieldPickerState{seq: m.copyFieldPicker.seq}
	if m.overlay == overlayCopyField {
		m.overlay = m.previousOverlay
		m.previousOverlay = overlayNone
	}
}

// recomputeCopyFieldVisible refreshes the filtered entry list for the
// active mode. Matching is case-insensitive over both the path and the
// value — so "ExternalIP" matches status.addresses[ExternalIP].address
// by path, and "34.1" matches it by value. Called only when the
// filter, mode, or entries change, so per-keystroke navigation never
// re-scans a large manifest (mirrors objectexplorer_find's
// recomputeFind).
func (m *Model) recomputeCopyFieldVisible() {
	p := &m.copyFieldPicker
	entries := m.copyFieldPickerCurrentEntries()
	f := strings.ToLower(p.filter)
	if f == "" {
		p.visible = entries
		return
	}
	out := make([]copyFieldEntry, 0, len(entries))
	for _, e := range entries {
		if strings.Contains(strings.ToLower(e.display), f) ||
			strings.Contains(strings.ToLower(e.value), f) {
			out = append(out, e)
		}
	}
	p.visible = out
}

// visibleCopyFieldEntries returns the cached filtered entry list.
func (m Model) visibleCopyFieldEntries() []copyFieldEntry {
	return m.copyFieldPicker.visible
}

// applyCopyFieldPicker copies the cursor row's value — from every item
// in the scope (columns mode) or every fetched manifest (fields mode),
// newline-joined in scope order — then closes the picker and records
// the entry as this kind's last copied.
func (m Model) applyCopyFieldPicker() (tea.Model, tea.Cmd) {
	if !m.copyFieldPicker.active {
		return m, nil
	}
	vis := m.visibleCopyFieldEntries()
	if m.copyFieldPicker.cursor < 0 || m.copyFieldPicker.cursor >= len(vis) {
		return m, nil
	}
	entry := vis[m.copyFieldPicker.cursor]
	mode := m.copyFieldPicker.mode
	kind := m.copyFieldPicker.kind

	var payload string
	var found, missing int
	if entry.column != "" {
		payload, found, missing = buildCopyFieldColumnPayload(m.copyFieldPicker.scope, entry.column)
	} else {
		payload, found, missing = buildCopyFieldPayload(m.copyFieldPicker.docs, entry.path)
	}
	m.closeCopyFieldPicker()

	if found == 0 {
		m.setStatusMessage("Copy field: no value for "+entry.display, true)
		return m, scheduleStatusClear()
	}
	if m.lastCopyFieldByKind == nil {
		m.lastCopyFieldByKind = make(map[string]copyFieldMemory)
	}
	m.lastCopyFieldByKind[kind] = copyFieldMemory{mode: mode, display: entry.display}

	status := "Copied " + entry.display
	if found > 1 || missing > 0 {
		status = fmt.Sprintf("Copied %d values for %s", found, entry.display)
		if missing > 0 {
			status += fmt.Sprintf(" (%d missing)", missing)
		}
	}
	m.setStatusMessage(status, false)
	return m, tea.Batch(copyToSystemClipboard(payload), scheduleStatusClear())
}

// buildCopyFieldColumnPayload extracts a column's cell from every scope
// item, newline-joined. Empty cells (and the "<none>" placeholder the
// table renderer uses) count as missing rather than emitting blanks.
func buildCopyFieldColumnPayload(scope []model.Item, column string) (payload string, found, missing int) {
	values := make([]string, 0, len(scope))
	for i := range scope {
		v := copyTableCellValue(&scope[i], column)
		if v == "" || v == "<none>" {
			missing++
			continue
		}
		values = append(values, v)
	}
	return strings.Join(values, "\n"), len(values), missing
}

// copyFieldOverlayDims returns the picker box width, height, and
// visible row count. Shared by the renderer and scroll clamp so the
// scrollbar and cursor stay in sync (same shape as findOverlayDims).
func (m Model) copyFieldOverlayDims() (w, h, maxVisible int) {
	w = min(m.width-6, max(m.width*70/100, 64))
	h = min(m.height-4, max(m.height*70/100, 12))
	maxVisible = max(h-8, 1) // title + subtitle + filter(2) + footer(2) + borders
	return w, h, maxVisible
}

// clampCopyFieldScroll keeps the cursor within the visible window.
func (m *Model) clampCopyFieldScroll() {
	p := &m.copyFieldPicker
	_, _, visible := m.copyFieldOverlayDims()
	n := len(m.visibleCopyFieldEntries())
	p.cursor = max(0, min(p.cursor, n-1))
	if p.cursor < p.scroll {
		p.scroll = p.cursor
	}
	if p.cursor >= p.scroll+visible {
		p.scroll = p.cursor - visible + 1
	}
	if p.scroll < 0 {
		p.scroll = 0
	}
}
