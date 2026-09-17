package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

// keymapItem is one row of the keymaps overlay: the underlying which-key
// entry rendered as an OverlayListItem, plus the raw (unformatted) key
// string needed to synthesize a keypress on Enter.
type keymapItem struct {
	ui.OverlayListItem
	rawKey string
}

// openKeymapsOverlay resets the overlay's cursor and filter and opens it.
// Value receiver: mirrors every other openX overlay helper in this package
// (e.g. previewSchemeAtCursor's siblings), so callers reassign `m`.
func (m Model) openKeymapsOverlay() Model {
	m.keymapsCursor = 0
	m.keymapsFilterMode = false
	m.keymapsFilter.Clear()
	m.overlay = overlayKeymaps
	return m
}

// keymapsOverlayItems builds the full (unfiltered) row list from the current
// mode's which-key catalog. availableWhichKeyActions already drops entries
// whose binding the user cleared, so every row here has a usable rawKey.
func (m *Model) keymapsOverlayItems() []keymapItem {
	actions := m.availableWhichKeyActions()
	items := make([]keymapItem, 0, len(actions))
	for _, a := range actions {
		raw := a.Key(ui.ActiveKeybindings)
		items = append(items, keymapItem{
			Key:         ui.KeyChordDisplay(raw),
			Name:        a.Label,
			Description: string(a.Group),
			rawKey:      raw,
		})
	}
	return items
}

// filteredKeymapsItems returns keymapsOverlayItems narrowed by the current
// filter text, matched against the label, group, and displayed key.
func (m *Model) filteredKeymapsItems() []keymapItem {
	items := m.keymapsOverlayItems()
	if m.keymapsFilter.Value == "" {
		return items
	}
	query := m.keymapsFilter.Value
	filtered := make([]keymapItem, 0, len(items))
	for _, it := range items {
		if ui.MatchLine(it.Name, query) || ui.MatchLine(it.Description, query) || ui.MatchLine(it.Key, query) {
			filtered = append(filtered, it)
		}
	}
	return filtered
}

// handleKeymapsOverlayKey routes keys to the filter or normal-mode handler
// for the keymaps overlay.
func (m Model) handleKeymapsOverlayKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.keymapsFilterMode {
		return m.handleKeymapsFilterMode(msg)
	}
	return m.handleKeymapsNormalMode(msg)
}

// dispatchKeymapsSelection closes the overlay and replays the selected
// entry's key as a synthetic keypress, so Enter has exactly the effect the
// real key would have had.
func dispatchKeymapsSelection(m Model, selected keymapItem) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.keymapsFilter.Clear()
	synthetic := parseKeyBinding(selected.rawKey)
	return m, func() tea.Msg { return synthetic }
}

func (m Model) handleKeymapsNormalMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredKeymapsItems()
	maxIdx := len(filtered) - 1

	switch msg.String() {
	case "esc", "q":
		if m.keymapsFilter.Value != "" {
			m.keymapsFilter.Clear()
			m.keymapsCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		return m, nil

	case "enter":
		if len(filtered) > 0 && m.keymapsCursor >= 0 && m.keymapsCursor <= maxIdx {
			return dispatchKeymapsSelection(m, filtered[m.keymapsCursor])
		}
		return m, nil

	case "/":
		m.keymapsFilterMode = true
		m.keymapsFilter.Clear()
		return m, nil

	case "j", "down", "ctrl+n":
		m.keymapsCursor = clampOverlayCursor(m.keymapsCursor, 1, maxIdx)
		return m, nil

	case "k", "up", "ctrl+p":
		m.keymapsCursor = clampOverlayCursor(m.keymapsCursor, -1, maxIdx)
		return m, nil

	case "ctrl+d", "shift+down":
		m.keymapsCursor = clampOverlayCursor(m.keymapsCursor, 10, maxIdx)
		return m, nil

	case "ctrl+u", "shift+up":
		m.keymapsCursor = clampOverlayCursor(m.keymapsCursor, -10, maxIdx)
		return m, nil

	case "ctrl+f", "pgdown":
		m.keymapsCursor = clampOverlayCursor(m.keymapsCursor, 20, maxIdx)
		return m, nil

	case "ctrl+b", "pgup":
		m.keymapsCursor = clampOverlayCursor(m.keymapsCursor, -20, maxIdx)
		return m, nil

	case "home":
		m.pendingG = false
		m.keymapsCursor = 0
		return m, nil

	case "end":
		m.pendingG = false
		if maxIdx >= 0 {
			m.keymapsCursor = maxIdx
		}
		return m, nil

	case "g":
		if m.pendingG {
			m.pendingG = false
			m.keymapsCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G":
		if maxIdx >= 0 {
			m.keymapsCursor = maxIdx
		}
		return m, nil
	}
	return m, nil
}

func (m Model) handleKeymapsFilterMode(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch handleFilterKey(&m.keymapsFilter, msg) {
	case filterEscape:
		m.keymapsFilterMode = false
		m.keymapsFilter.Clear()
		m.keymapsCursor = 0
		return m, nil

	case filterAccept:
		m.keymapsFilterMode = false
		filtered := m.filteredKeymapsItems()
		// A single match makes Enter unambiguous: dispatch it now instead of
		// making the user press Enter a second time to leave filter mode.
		if len(filtered) == 1 {
			return dispatchKeymapsSelection(m, filtered[0])
		}
		m.keymapsCursor = 0
		return m, nil

	case filterClose:
		return m.closeTabOrQuit()

	case filterContinue:
		m.keymapsCursor = 0
		return m, nil
	}
	return m, nil
}
