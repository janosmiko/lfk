package app

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// builtinColumnOrder is the canonical left-to-right order of built-in
// columns in RenderTable. openColumnToggle emits entries in this order so
// the overlay matches the header row. Name leads the list: it renders first
// by default but is reorderable and hideable like any other column.
var builtinColumnOrder = []string{"Name", "Context", "Namespace", "Ready", "Restarts", "Status", "Age"}

// columnToggleSnapshot captures the pre-overlay column-config state for
// a single kind so Esc can revert after live-apply edits. The hasX
// flags distinguish "no map entry" (auto-detect) from "explicit empty
// slice" (user said no extras), which sessionColumns relies on.
type columnToggleSnapshot struct {
	kind       string
	session    []string
	hasSession bool
	hidden     []string
	hasHidden  bool
	order      []string
	hasOrder   bool
}

// captureColumnToggleSnapshot deep-copies the current per-kind column
// config so a later restoreColumnToggleSnapshot can undo any live-apply
// edits the user made while the overlay was open.
func captureColumnToggleSnapshot(m *Model, kind string) columnToggleSnapshot {
	snap := columnToggleSnapshot{kind: kind}
	key := m.columnMemoryKey(kind)
	if v, ok := m.sessionColumns[key]; ok {
		snap.hasSession = true
		snap.session = append([]string(nil), v...)
	}
	if v, ok := m.hiddenBuiltinColumns[key]; ok {
		snap.hasHidden = true
		snap.hidden = append([]string(nil), v...)
	}
	if v, ok := m.columnOrder[key]; ok {
		snap.hasOrder = true
		snap.order = append([]string(nil), v...)
	}
	return snap
}

// restoreColumnToggleSnapshot puts the maps back to the snapshot taken
// at overlay open. Called when the user presses Esc to discard the
// live-applied changes.
func restoreColumnToggleSnapshot(m *Model, snap columnToggleSnapshot) {
	if snap.kind == "" {
		return
	}
	// Context cannot change while the overlay is open, so recompute the
	// context-scoped key from the captured kind.
	key := m.columnMemoryKey(snap.kind)
	if snap.hasSession {
		if m.sessionColumns == nil {
			m.sessionColumns = make(map[string][]string)
		}
		m.sessionColumns[key] = snap.session
	} else {
		delete(m.sessionColumns, key)
	}
	if snap.hasHidden {
		if m.hiddenBuiltinColumns == nil {
			m.hiddenBuiltinColumns = make(map[string][]string)
		}
		m.hiddenBuiltinColumns[key] = snap.hidden
	} else {
		delete(m.hiddenBuiltinColumns, key)
	}
	if snap.hasOrder {
		if m.columnOrder == nil {
			m.columnOrder = make(map[string][]string)
		}
		m.columnOrder[key] = snap.order
	} else {
		delete(m.columnOrder, key)
	}
}

// openColumnToggle populates the column toggle overlay from the current
// resource list. It enumerates both built-in columns (backed by Item fields)
// and extra columns (from Item.Columns), pre-selecting entries to reflect
// what is currently rendered on screen.
func (m *Model) openColumnToggle() {
	items := m.visibleMiddleItems()
	// Use middleColumnKind so column config at LevelContainers/LevelOwned
	// is scoped to the actual kind being shown (e.g., "container", not the
	// parent pod's "pod").
	kind := m.middleColumnKind()
	key := m.columnMemoryKey(kind)

	builtinEntries := m.collectBuiltinToggleEntries(items, kind)
	extraEntries := m.collectExtraToggleEntries(items)

	if len(builtinEntries) == 0 && len(extraEntries) == 0 {
		return
	}

	entries := mergeColumnToggleEntries(builtinEntries, extraEntries, m.columnOrder[key])

	m.columnToggleItems = entries
	m.columnToggleCursor = 0
	m.columnToggleFilter = ""
	m.columnToggleFilterActive = false
	// Snapshot the current per-kind config so Esc can undo the
	// live-applied edits the user is about to make in the overlay.
	m.columnToggleSnapshot = captureColumnToggleSnapshot(m, kind)
	m.overlay = overlayColumnToggle
}

// mergeColumnToggleEntries interleaves built-in and extra toggle entries
// according to the user's saved column order for the current kind. Entries
// whose keys are not listed in savedOrder are appended in the default
// position (built-ins first, then extras) so newly-discovered columns still
// surface. Built-in keys take precedence over any extra sharing the name.
func mergeColumnToggleEntries(builtinEntries, extraEntries []columnToggleEntry, savedOrder []string) []columnToggleEntry {
	byKey := make(map[string]columnToggleEntry, len(builtinEntries)+len(extraEntries))
	for _, e := range builtinEntries {
		byKey[e.key] = e
	}
	for _, e := range extraEntries {
		// Built-in keys take precedence — never let an extra with a clashing key overwrite.
		if _, exists := byKey[e.key]; exists {
			continue
		}
		byKey[e.key] = e
	}

	result := make([]columnToggleEntry, 0, len(byKey))
	seen := make(map[string]bool, len(byKey))

	// Backward compatibility: saved orders that predate configurable Name omit
	// the "Name" key. Pin Name first so reopening the overlay matches the
	// renderer's pinned-first behaviour in orderedColumnKeys; without this,
	// reopening a legacy order and pressing Enter would persist Name at a
	// non-first position. An order that lists "Name" honours that position.
	if e, ok := byKey["Name"]; ok && !slices.Contains(savedOrder, "Name") {
		result = append(result, e)
		seen["Name"] = true
	}

	for _, k := range savedOrder {
		if e, ok := byKey[k]; ok && !seen[k] {
			result = append(result, e)
			seen[k] = true
		}
	}
	// Append remaining entries in default order: built-ins first, then extras.
	for _, e := range builtinEntries {
		if !seen[e.key] {
			result = append(result, e)
			seen[e.key] = true
		}
	}
	for _, e := range extraEntries {
		if !seen[e.key] {
			result = append(result, e)
			seen[e.key] = true
		}
	}
	return result
}

// collectBuiltinToggleEntries returns toggle entries for built-in columns
// present in the item data. Name is always present (every item has a name) and
// is reorderable/hideable like the others. The visible flag is true unless the
// column is in hiddenBuiltinColumns[kind].
func (m *Model) collectBuiltinToggleEntries(items []model.Item, kind string) []columnToggleEntry {
	present := map[string]bool{"Name": true}
	for _, item := range items {
		if item.ClusterName != "" {
			present["Context"] = true
		}
		if item.Namespace != "" {
			present["Namespace"] = true
		}
		if item.Ready != "" {
			present["Ready"] = true
		}
		if item.Restarts != "" {
			present["Restarts"] = true
		}
		if item.Status != "" {
			present["Status"] = true
		}
		if item.Age != "" {
			present["Age"] = true
		}
	}

	// Effective hidden state: session toggle wins over view-derived defaults.
	// Without this, opening the overlay on a kind that has a view: configured
	// would show all built-ins as "visible" even though the table is hiding
	// the ones not listed in the view — and the first toggle would silently
	// commit that wrong baseline as session state, wiping the view's effect.
	hidden := map[string]bool{}
	if sessionHidden, ok := m.hiddenBuiltinColumns[m.columnMemoryKey(kind)]; ok && len(sessionHidden) > 0 {
		for _, k := range sessionHidden {
			hidden[k] = true
		}
	} else if viewHidden := ui.HiddenBuiltinsForView(m.viewRefForKind(kind), m.nav.Context); viewHidden != nil {
		for k := range viewHidden {
			hidden[k] = true
		}
	}

	entries := make([]columnToggleEntry, 0, len(builtinColumnOrder))
	for _, key := range builtinColumnOrder {
		if !present[key] {
			continue
		}
		entries = append(entries, columnToggleEntry{
			key:     key,
			visible: !hidden[key],
			builtin: true,
		})
	}
	return entries
}

// collectExtraToggleEntries returns toggle entries for extra columns found in
// item.Columns. Visibility is derived from ui.ActiveExtraColumnKeys (the set
// of extra columns currently rendered on screen) so the overlay always
// reflects what the user actually sees regardless of saved preferences.
func (m *Model) collectExtraToggleEntries(items []model.Item) []columnToggleEntry {
	seen := make(map[string]bool)
	var allKeys []string
	for _, item := range items {
		for _, kv := range item.Columns {
			if strings.HasPrefix(kv.Key, "__") || strings.HasPrefix(kv.Key, "secret:") ||
				strings.HasPrefix(kv.Key, "owner:") || strings.HasPrefix(kv.Key, "data:") ||
				strings.HasPrefix(kv.Key, "condition:") || strings.HasPrefix(kv.Key, "step:") ||
				strings.HasPrefix(kv.Key, "cond:") {
				continue
			}
			if !seen[kv.Key] {
				seen[kv.Key] = true
				allKeys = append(allKeys, kv.Key)
			}
		}
	}

	if len(allKeys) == 0 {
		return nil
	}

	visibleSet := make(map[string]bool, len(ui.ActiveExtraColumnKeys))
	for _, k := range ui.ActiveExtraColumnKeys {
		visibleSet[k] = true
	}

	// Visible extras first, in the order they appear on screen.
	entries := make([]columnToggleEntry, 0, len(allKeys))
	for _, k := range ui.ActiveExtraColumnKeys {
		if seen[k] {
			entries = append(entries, columnToggleEntry{key: k, visible: true})
		}
	}
	// Then hidden extras, in discovery order.
	for _, k := range allKeys {
		if !visibleSet[k] {
			entries = append(entries, columnToggleEntry{key: k, visible: false})
		}
	}
	return entries
}

// handleColumnToggleKey handles keyboard input for the column toggle overlay.
func (m Model) handleColumnToggleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.columnToggleFilterActive {
		return m.handleColumnToggleFilterKey(msg)
	}

	items := m.filteredColumnToggleItems()
	maxIdx := len(items) - 1

	switch msg.String() {
	case "esc", "q":
		return m.handleColumnToggleKeyEsc()

	case "j", "down":
		if m.columnToggleCursor < maxIdx {
			m.columnToggleCursor++
		}
		return m, nil

	case "k", "up":
		return m.handleColumnToggleKeyK()

	case "ctrl+d":
		m.columnToggleCursor = clampOverlayCursor(m.columnToggleCursor, 10, maxIdx)
		return m, nil

	case "ctrl+u":
		m.columnToggleCursor = clampOverlayCursor(m.columnToggleCursor, -10, maxIdx)
		return m, nil

	case "ctrl+f":
		m.columnToggleCursor = clampOverlayCursor(m.columnToggleCursor, 20, maxIdx)
		return m, nil

	case "ctrl+b":
		m.columnToggleCursor = clampOverlayCursor(m.columnToggleCursor, -20, maxIdx)
		return m, nil

	case " ":
		// Toggle visibility, persist live, then advance cursor.
		if m.columnToggleCursor >= 0 && m.columnToggleCursor < len(items) {
			key := items[m.columnToggleCursor].key
			for i := range m.columnToggleItems {
				if m.columnToggleItems[i].key == key {
					m.columnToggleItems[i].visible = !m.columnToggleItems[i].visible
					break
				}
			}
		}
		m.applyColumnToggleState()
		if m.columnToggleCursor < maxIdx {
			m.columnToggleCursor++
		}
		return m, nil

	case "J":
		// Move column down in priority.
		return m.handleColumnToggleKeyJ()

	case "K":
		// Move column up in priority.
		return m.handleColumnToggleKeyK2()

	case "enter":
		// Apply: save visible columns in order to session state.
		return m.handleColumnToggleKeyEnter()

	case "/":
		m.columnToggleFilterActive = true
		return m, nil

	case "R":
		// Reset: clear session override, fall back to config/auto-detect.
		return m.handleColumnToggleKeyR()

	case "c":
		// Clear: uncheck every entry without closing the overlay so the
		// user can pick a fresh set. Live-apply persists the empty state
		// — Esc reverts via the snapshot, Enter just closes.
		for i := range m.columnToggleItems {
			m.columnToggleItems[i].visible = false
		}
		m.applyColumnToggleState()
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}

	return m, nil
}

// handleColumnToggleFilterKey handles text input in the column toggle filter.
func (m Model) handleColumnToggleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		if m.columnToggleFilter != "" {
			m.columnToggleFilter = ""
			m.columnToggleCursor = 0
		} else {
			m.columnToggleFilterActive = false
		}
		return m, nil
	case "enter":
		m.columnToggleFilterActive = false
		return m, nil
	case "backspace":
		if len(m.columnToggleFilter) > 0 {
			m.columnToggleFilter = m.columnToggleFilter[:len(m.columnToggleFilter)-1]
			m.columnToggleCursor = 0
		}
		return m, nil
	case "ctrl+w":
		f := strings.TrimRight(m.columnToggleFilter, " ")
		if idx := strings.LastIndex(f, " "); idx >= 0 {
			m.columnToggleFilter = f[:idx+1]
		} else {
			m.columnToggleFilter = ""
		}
		m.columnToggleCursor = 0
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.columnToggleFilter += key
			m.columnToggleCursor = 0
		}
		return m, nil
	}
}

// filteredColumnToggleItems returns column toggle entries matching the filter.
func (m *Model) filteredColumnToggleItems() []columnToggleEntry {
	if m.columnToggleFilter == "" {
		return m.columnToggleItems
	}
	rawQuery := m.columnToggleFilter
	var filtered []columnToggleEntry
	for _, e := range m.columnToggleItems {
		if ui.MatchLine(e.key, rawQuery) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

func (m Model) handleColumnToggleKeyEsc() (tea.Model, tea.Cmd) {
	if m.columnToggleFilter != "" {
		m.columnToggleFilter = ""
		m.columnToggleCursor = 0
		return m, nil
	}
	// Discard live-applied edits by reverting to the snapshot taken at
	// openColumnToggle. Without this, Esc would silently keep whatever
	// the user toggled while exploring.
	restoreColumnToggleSnapshot(&m, m.columnToggleSnapshot)
	m.overlay = overlayNone
	m.columnToggleItems = nil
	m.columnToggleSnapshot = columnToggleSnapshot{}
	return m, nil
}

func (m Model) handleColumnToggleKeyK() (tea.Model, tea.Cmd) {
	if m.columnToggleCursor > 0 {
		m.columnToggleCursor--
	}
	return m, nil
}

func (m Model) handleColumnToggleKeyJ() (tea.Model, tea.Cmd) {
	if m.columnToggleFilter != "" {
		return m, nil // no reorder while filtering
	}
	if m.columnToggleCursor < len(m.columnToggleItems)-1 {
		i := m.columnToggleCursor
		m.columnToggleItems[i], m.columnToggleItems[i+1] = m.columnToggleItems[i+1], m.columnToggleItems[i]
		m.columnToggleCursor++
		m.applyColumnToggleState()
	}
	return m, nil
}

func (m Model) handleColumnToggleKeyK2() (tea.Model, tea.Cmd) {
	if m.columnToggleFilter != "" {
		return m, nil
	}
	if m.columnToggleCursor > 0 {
		i := m.columnToggleCursor
		m.columnToggleItems[i], m.columnToggleItems[i-1] = m.columnToggleItems[i-1], m.columnToggleItems[i]
		m.columnToggleCursor--
		m.applyColumnToggleState()
	}
	return m, nil
}

// applyColumnToggleState writes the overlay's current entry list to
// sessionColumns / hiddenBuiltinColumns / columnOrder for the current
// kind WITHOUT closing the overlay. Called after every Space/J/K/c so
// the table behind the overlay reflects edits live. handleColumnToggleKeyEnter
// is now just "apply + close"; Esc reverts via restoreColumnToggleSnapshot.
//
// Pointer receiver because we may need to assign newly-allocated maps
// when the caller's map fields are nil — a value receiver would only
// update the local copy.
func (m *Model) applyColumnToggleState() {
	key := m.columnMemoryKey(m.middleColumnKind())

	var visibleExtras []string
	var hiddenBuiltins []string
	var fullOrder []string
	visibleCount := 0
	for _, e := range m.columnToggleItems {
		if e.visible {
			visibleCount++
			fullOrder = append(fullOrder, e.key)
		}
		if e.builtin {
			if !e.visible {
				hiddenBuiltins = append(hiddenBuiltins, e.key)
			}
			continue
		}
		if e.visible {
			visibleExtras = append(visibleExtras, e.key)
		}
	}

	// Total clear: drop all per-kind entries so the table renders defaults.
	// The behavior matches a hypothetical R reset, but the overlay STAYS
	// open under the new live-apply contract — only Enter or Esc close it.
	if visibleCount == 0 {
		delete(m.sessionColumns, key)
		delete(m.hiddenBuiltinColumns, key)
		delete(m.columnOrder, key)
		return
	}

	if m.sessionColumns == nil {
		m.sessionColumns = make(map[string][]string)
	}
	// Non-nil empty slice = "user explicitly configured no extras" so the
	// auto-detect path doesn't re-add CPU/MEM next render.
	if visibleExtras == nil {
		visibleExtras = []string{}
	}
	m.sessionColumns[key] = visibleExtras

	if m.hiddenBuiltinColumns == nil {
		m.hiddenBuiltinColumns = make(map[string][]string)
	}
	if len(hiddenBuiltins) == 0 {
		delete(m.hiddenBuiltinColumns, key)
	} else {
		m.hiddenBuiltinColumns[key] = hiddenBuiltins
	}

	if m.columnOrder == nil {
		m.columnOrder = make(map[string][]string)
	}
	if len(fullOrder) == 0 {
		delete(m.columnOrder, key)
	} else {
		m.columnOrder[key] = fullOrder
	}
}

func (m Model) handleColumnToggleKeyEnter() (tea.Model, tea.Cmd) {
	// Idempotent commit: live-apply already wrote the same state on each
	// Space/J/K/c, so calling apply here is usually a no-op — but it also
	// covers programmatic flows that mutate columnToggleItems directly
	// without going through the keystroke handlers. Then close and drop
	// the snapshot so a future Esc-after-reopen doesn't restore stale
	// state.
	m.applyColumnToggleState()
	// Persist the committed layout (issue #353 follow-up). Live-apply during
	// the overlay only touches in-memory maps; the on-disk layout is written
	// here on commit so an Esc that reverts via the snapshot never leaks an
	// uncommitted edit to disk.
	m.persistColumnPrefs()
	m.overlay = overlayNone
	m.columnToggleItems = nil
	m.columnToggleSnapshot = columnToggleSnapshot{}
	return m, nil
}

func (m Model) handleColumnToggleKeyR() (tea.Model, tea.Cmd) {
	key := m.columnMemoryKey(m.middleColumnKind())
	if m.sessionColumns != nil {
		delete(m.sessionColumns, key)
	}
	if m.hiddenBuiltinColumns != nil {
		delete(m.hiddenBuiltinColumns, key)
	}
	if m.columnOrder != nil {
		delete(m.columnOrder, key)
	}
	// Drop the persisted layout too, so the reset survives a restart.
	m.persistColumnPrefs()
	m.overlay = overlayNone
	m.columnToggleItems = nil
	// Match Enter: drop the snapshot so a future reopen can't restore stale
	// pre-reset state.
	m.columnToggleSnapshot = columnToggleSnapshot{}
	m.setStatusMessage("Columns reset to default", false)
	return m, scheduleStatusClear()
}
