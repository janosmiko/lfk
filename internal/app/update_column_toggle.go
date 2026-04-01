package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// openColumnToggle populates the column toggle overlay from the current resource list.
func (m *Model) openColumnToggle() {
	// Collect all available column keys from current items.
	items := m.visibleMiddleItems()
	seen := make(map[string]bool)
	var allKeys []string
	for _, item := range items {
		for _, kv := range item.Columns {
			// Skip internal/data prefixed columns.
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
		return
	}

	// Determine which columns are currently visible.
	// Use session override if set, otherwise get from collectExtraColumns result.
	kind := strings.ToLower(m.nav.ResourceType.Kind)
	var visibleKeys map[string]bool
	if sessionCols, ok := m.sessionColumns[kind]; ok {
		visibleKeys = make(map[string]bool, len(sessionCols))
		for _, k := range sessionCols {
			visibleKeys[k] = true
		}
	} else {
		// Use currently displayed columns from the table.
		visibleKeys = make(map[string]bool)
		for _, k := range ui.ActiveExtraColumnKeys {
			visibleKeys[k] = true
		}
	}

	// Build entries: visible ones first (in order), then hidden ones.
	var entries []columnToggleEntry
	// Add visible columns in their current order.
	for _, k := range allKeys {
		if visibleKeys[k] {
			entries = append(entries, columnToggleEntry{key: k, visible: true})
		}
	}
	// Add hidden columns after.
	for _, k := range allKeys {
		if !visibleKeys[k] {
			entries = append(entries, columnToggleEntry{key: k, visible: false})
		}
	}

	m.columnToggleItems = entries
	m.columnToggleCursor = 0
	m.columnToggleFilter = ""
	m.columnToggleFilterActive = false
	m.overlay = overlayColumnToggle
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
		if m.columnToggleFilter != "" {
			m.columnToggleFilter = ""
			m.columnToggleCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		m.columnToggleItems = nil
		return m, nil

	case "j", "down":
		if m.columnToggleCursor < maxIdx {
			m.columnToggleCursor++
		}
		return m, nil

	case "k", "up":
		if m.columnToggleCursor > 0 {
			m.columnToggleCursor--
		}
		return m, nil

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
		if m.columnToggleFilter != "" {
			return m, nil // no reorder while filtering
		}
		if m.columnToggleCursor < len(m.columnToggleItems)-1 {
			i := m.columnToggleCursor
			m.columnToggleItems[i], m.columnToggleItems[i+1] = m.columnToggleItems[i+1], m.columnToggleItems[i]
			m.columnToggleCursor++
		}
		return m, nil

	case "K":
		// Move column up in priority.
		if m.columnToggleFilter != "" {
			return m, nil
		}
		if m.columnToggleCursor > 0 {
			i := m.columnToggleCursor
			m.columnToggleItems[i], m.columnToggleItems[i-1] = m.columnToggleItems[i-1], m.columnToggleItems[i]
			m.columnToggleCursor--
		}
		return m, nil

	case "enter":
		// Apply: save visible columns in order to session state.
		kind := strings.ToLower(m.nav.ResourceType.Kind)
		var visible []string
		for _, e := range m.columnToggleItems {
			if e.visible {
				visible = append(visible, e.key)
			}
		}
		if m.sessionColumns == nil {
			m.sessionColumns = make(map[string][]string)
		}
		if len(visible) == 0 {
			delete(m.sessionColumns, kind)
		} else {
			m.sessionColumns[kind] = visible
		}
		m.overlay = overlayNone
		m.columnToggleItems = nil
		return m, nil

	case "/":
		m.columnToggleFilterActive = true
		return m, nil

	case "R":
		// Reset: clear session override, fall back to config/auto-detect.
		kind := strings.ToLower(m.nav.ResourceType.Kind)
		if m.sessionColumns != nil {
			delete(m.sessionColumns, kind)
		}
		m.overlay = overlayNone
		m.columnToggleItems = nil
		m.setStatusMessage("Columns reset to default", false)
		return m, scheduleStatusClear()

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
	lower := strings.ToLower(m.columnToggleFilter)
	var filtered []columnToggleEntry
	for _, e := range m.columnToggleItems {
		if strings.Contains(strings.ToLower(e.key), lower) {
			filtered = append(filtered, e)
		}
	}
	return filtered
}
