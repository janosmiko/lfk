package app

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

type keymapItem struct {
	ui.OverlayListItem
	rawKey     string
	displayKey string
}

// openKeymapsOverlay resets the overlay's cursor and filter and opens it.
// Value receiver: mirrors every other openX overlay helper in this package
// (e.g. previewSchemeAtCursor's siblings), so callers reassign `m`.
func (m Model) openKeymapsOverlay() Model {
	m.keymapsCursor = 0
	m.keymapsFilterMode = true
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
		group := string(a.Group)
		key := ui.KeyChordDisplay(raw)
		items = append(items, keymapItem{ //nolint:modernize // can't drop embedded label with named fields
			OverlayListItem: ui.OverlayListItem{
				Name:        a.Label,
				Description: group,
				Badge:       keymapsBadge(group, key),
			},
			rawKey:     raw,
			displayKey: key,
		})
	}
	return items
}

const (
	keymapsGroupW = 9 // len("Selection"), the longest group name
	keymapsKeyW   = 6
	keymapsBadgeW = keymapsGroupW + 2 + keymapsKeyW + 1 // group + gap + key + scrollbar pad
)

func keymapsBadge(group, key string) string {
	gs := keymapsGroupStyle(whichKeyGroup(group))
	ks := lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorSecondary)).Background(ui.SurfaceBg).Bold(true)
	return gs.Render(fmt.Sprintf("%-*s", keymapsGroupW, group)) +
		lipgloss.NewStyle().Background(ui.SurfaceBg).Render("  ") +
		ks.Render(fmt.Sprintf("%*s", keymapsKeyW, key)) +
		lipgloss.NewStyle().Background(ui.SurfaceBg).Render(" ")
}

func keymapsGroupStyle(g whichKeyGroup) lipgloss.Style {
	var base lipgloss.Style
	switch g {
	case wkActions:
		base = ui.WhichKeyActionsStyle
	case wkViews:
		base = ui.WhichKeyViewsStyle
	case wkFilter:
		base = ui.WhichKeyFilterStyle
	case wkSelection:
		base = ui.WhichKeySelectionStyle
	case wkSort:
		base = ui.WhichKeySortStyle
	case wkSettings:
		base = ui.WhichKeySettingsStyle
	default:
		base = lipgloss.NewStyle().Foreground(lipgloss.Color(ui.ColorDimmed))
	}
	return base.Background(ui.SurfaceBg)
}

func (m *Model) filteredKeymapsItems() []keymapItem {
	return keymapsFilter(m.keymapsOverlayItems(), m.keymapsFilter.Value)
}

func keymapsFilter(items []keymapItem, query string) []keymapItem {
	if query == "" {
		return items
	}
	filtered := make([]keymapItem, 0, len(items))
	for _, it := range items {
		if ui.MatchLine(it.Name, query) || ui.MatchLine(it.Description, query) || ui.MatchLine(it.displayKey, query) {
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
