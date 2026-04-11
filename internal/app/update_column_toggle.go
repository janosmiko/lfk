package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// builtinColumnOrder is the canonical left-to-right order of built-in
// columns in RenderTable. openColumnToggle emits entries in this order so
// the overlay matches the header row.
var builtinColumnOrder = []string{"Namespace", "Ready", "Restarts", "Status", "Age"}

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

	builtinEntries := m.collectBuiltinToggleEntries(items, kind)
	extraEntries := m.collectExtraToggleEntries(items)

	if len(builtinEntries) == 0 && len(extraEntries) == 0 {
		return
	}

	entries := mergeColumnToggleEntries(builtinEntries, extraEntries, m.columnOrder[kind])

	m.columnToggleItems = entries
	m.columnToggleCursor = 0
	m.columnToggleFilter = ""
	m.columnToggleFilterActive = false
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
// present in the item data. Name is intentionally excluded (it is mandatory).
// The visible flag is true unless the column is in hiddenBuiltinColumns[kind].
func (m *Model) collectBuiltinToggleEntries(items []model.Item, kind string) []columnToggleEntry {
	present := map[string]bool{}
	for _, item := range items {
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

	hidden := map[string]bool{}
	for _, k := range m.hiddenBuiltinColumns[kind] {
		hidden[k] = true
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
		// Toggle visibility and advance cursor.
		if m.columnToggleCursor >= 0 && m.columnToggleCursor < len(items) {
			key := items[m.columnToggleCursor].key
			for i := range m.columnToggleItems {
				if m.columnToggleItems[i].key == key {
					m.columnToggleItems[i].visible = !m.columnToggleItems[i].visible
					break
				}
			}
		}
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
		// Clear: uncheck every entry without closing the overlay, so the
		// user can pick a fresh set from scratch. Apply with Enter to save.
		for i := range m.columnToggleItems {
			m.columnToggleItems[i].visible = false
		}
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
	m.overlay = overlayNone
	m.columnToggleItems = nil
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
	}
	return m, nil
}

func (m Model) handleColumnToggleKeyEnter() (tea.Model, tea.Cmd) {
	kind := m.middleColumnKind()

	// Walk the current overlay order to collect three parallel views of the
	// user's selection:
	//   - visibleExtras: ordered visible extras (persisted to sessionColumns)
	//   - hiddenBuiltins: built-in keys the user unchecked
	//   - fullOrder: all visible entries in overlay order (persisted to
	//     columnOrder so the next render honors the user's interleaving)
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

	// If the user unselected absolutely everything, interpret it as "revert
	// to defaults" instead of leaving the table completely empty. This is
	// equivalent to pressing R and matches the more intuitive behavior.
	if visibleCount == 0 {
		delete(m.sessionColumns, kind)
		delete(m.hiddenBuiltinColumns, kind)
		delete(m.columnOrder, kind)
		m.overlay = overlayNone
		m.columnToggleItems = nil
		return m, nil
	}

	if m.sessionColumns == nil {
		m.sessionColumns = make(map[string][]string)
	}
	// Always save the extras list, even if empty, so the user's explicit
	// "no extras" choice is preserved. A non-nil empty slice is the signal
	// that the user configured this kind; selectColumnCandidates treats nil
	// (no entry in the map) as "auto-detect" and an empty slice as "show
	// no extras". Without this, unselecting all extras would fall back to
	// auto-detect and re-add columns like CPU/MEM.
	if visibleExtras == nil {
		visibleExtras = []string{}
	}
	m.sessionColumns[kind] = visibleExtras

	if m.hiddenBuiltinColumns == nil {
		m.hiddenBuiltinColumns = make(map[string][]string)
	}
	if len(hiddenBuiltins) == 0 {
		delete(m.hiddenBuiltinColumns, kind)
	} else {
		m.hiddenBuiltinColumns[kind] = hiddenBuiltins
	}

	if m.columnOrder == nil {
		m.columnOrder = make(map[string][]string)
	}
	if len(fullOrder) == 0 {
		delete(m.columnOrder, kind)
	} else {
		m.columnOrder[kind] = fullOrder
	}

	m.overlay = overlayNone
	m.columnToggleItems = nil
	return m, nil
}

func (m Model) handleColumnToggleKeyR() (tea.Model, tea.Cmd) {
	kind := m.middleColumnKind()
	if m.sessionColumns != nil {
		delete(m.sessionColumns, kind)
	}
	if m.hiddenBuiltinColumns != nil {
		delete(m.hiddenBuiltinColumns, kind)
	}
	if m.columnOrder != nil {
		delete(m.columnOrder, kind)
	}
	m.overlay = overlayNone
	m.columnToggleItems = nil
	m.setStatusMessage("Columns reset to default", false)
	return m, scheduleStatusClear()
}
