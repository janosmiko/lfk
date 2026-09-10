package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleFinalizerSearchKey handles keyboard input for the finalizer search overlay.
func (m Model) handleFinalizerSearchKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// When filter input is active, handle text input first.
	if m.finalizerSearch.filterActive {
		return m.handleFinalizerSearchFilterKey(msg)
	}

	filtered := m.filteredFinalizerResults()
	maxIdx := len(filtered) - 1

	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		m.finalizerSearch.results = nil
		m.finalizerSearch.selected = nil
		m.finalizerSearch.filter = ""
		m.finalizerSearch.filterActive = false
		return m, nil

	case "j", "down":
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, 1, maxIdx)
		return m, nil

	case "k", "up":
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, -1, maxIdx)
		return m, nil

	case "g":
		// gg to top.
		if m.pendingG {
			m.pendingG = false
			m.finalizerSearch.cursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G":
		m.finalizerSearch.cursor = maxIdx
		return m, nil

	case "ctrl+d", "shift+down":
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, 10, maxIdx)
		return m, nil

	case "ctrl+u", "shift+up":
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, -10, maxIdx)
		return m, nil

	case "ctrl+f":
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, 20, maxIdx)
		return m, nil

	case "ctrl+b":
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, -20, maxIdx)
		return m, nil

	case "space":
		// Toggle selection on the current item and advance cursor.
		if m.finalizerSearch.cursor >= 0 && m.finalizerSearch.cursor < len(filtered) {
			match := filtered[m.finalizerSearch.cursor]
			k := finalizerMatchKey(match)
			if m.finalizerSearch.selected[k] {
				delete(m.finalizerSearch.selected, k)
			} else {
				m.finalizerSearch.selected[k] = true
			}
		}
		m.finalizerSearch.cursor = clampOverlayCursor(m.finalizerSearch.cursor, 1, maxIdx)
		return m, nil

	case "ctrl+a":
		// Select/deselect all visible (filtered) results.
		allSelected := true
		for _, match := range filtered {
			if !m.finalizerSearch.selected[finalizerMatchKey(match)] {
				allSelected = false
				break
			}
		}
		if allSelected {
			// Deselect all.
			for _, match := range filtered {
				delete(m.finalizerSearch.selected, finalizerMatchKey(match))
			}
		} else {
			// Select all.
			for _, match := range filtered {
				m.finalizerSearch.selected[finalizerMatchKey(match)] = true
			}
		}
		return m, nil

	case "enter":
		// Confirm removal: open a type-to-confirm overlay.
		selectedCount := len(m.finalizerSearch.selected)
		if selectedCount == 0 {
			m.setStatusMessage("No resources selected", true)
			return m, scheduleStatusClear()
		}
		m.confirmTitle = "Remove Finalizer"
		m.confirmQuestion = fmt.Sprintf(
			"Remove finalizer from %d resource(s)? Type DELETE to confirm.",
			selectedCount,
		)
		m.pendingAction = "Finalizer Remove"
		m.overlay = overlayConfirmType
		m.confirmTypeInput.Clear()
		return m, nil

	case "/":
		m.finalizerSearch.filterActive = true
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}

	return m, nil
}

// handleFinalizerSearchFilterKey handles keyboard input when the filter bar
// is active inside the finalizer search overlay.
func (m Model) handleFinalizerSearchFilterKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Pasted text arrives as a single key event with Paste=true and the
	// pasted content in Runes. Append single-line pastes to the filter;
	// drop multi-line pastes (the finalizer filter is single-line by
	// design, so collapsing newlines would silently hide content).
	if isPaste(msg) {
		pasted := strings.TrimRight(msg.Text, "\n")
		if !strings.ContainsAny(pasted, "\n\r") {
			m.finalizerSearch.filter += pasted
			m.finalizerSearch.cursor = 0
		}
		return m, nil
	}
	key := msg.String()
	switch key {
	case "esc":
		if m.finalizerSearch.filter != "" {
			m.finalizerSearch.filter = ""
			m.finalizerSearch.cursor = 0
		} else if m.finalizerSearch.results == nil && m.finalizerSearch.pattern == "" {
			// No search performed yet — close the overlay entirely.
			m.finalizerSearch.filterActive = false
			m.overlay = overlayNone
		} else {
			m.finalizerSearch.filterActive = false
		}
		return m, nil
	case "enter":
		if m.finalizerSearch.results == nil && m.finalizerSearch.pattern == "" {
			// Initial search prompt: use filter text as the search pattern.
			pattern := strings.TrimSpace(m.finalizerSearch.filter)
			if pattern == "" {
				return m, nil
			}
			m.finalizerSearch.pattern = pattern
			m.finalizerSearch.filter = ""
			m.finalizerSearch.filterActive = false
			m.finalizerSearch.loading = true
			return m, m.searchFinalizers(pattern)
		}
		m.finalizerSearch.filterActive = false
		return m, nil
	case "backspace":
		if len(m.finalizerSearch.filter) > 0 {
			m.finalizerSearch.filter = m.finalizerSearch.filter[:len(m.finalizerSearch.filter)-1]
			m.finalizerSearch.cursor = 0
		}
		return m, nil
	case "ctrl+w":
		// Delete word.
		f := m.finalizerSearch.filter
		f = strings.TrimRight(f, " ")
		if idx := strings.LastIndex(f, " "); idx >= 0 {
			m.finalizerSearch.filter = f[:idx+1]
		} else {
			m.finalizerSearch.filter = ""
		}
		m.finalizerSearch.cursor = 0
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		if msg.Text != "" {
			m.finalizerSearch.filter += msg.Text
			m.finalizerSearch.cursor = 0
		}
		return m, nil
	}
}

// filteredFinalizerResults returns the finalizer search results filtered
// by the current filter text (matching name, namespace, or kind).
func (m Model) filteredFinalizerResults() []k8s.FinalizerMatch {
	if m.finalizerSearch.filter == "" {
		return m.finalizerSearch.results
	}
	rawQuery := m.finalizerSearch.filter
	var filtered []k8s.FinalizerMatch
	for _, r := range m.finalizerSearch.results {
		if ui.MatchLine(r.Name, rawQuery) ||
			ui.MatchLine(r.Namespace, rawQuery) ||
			ui.MatchLine(r.Kind, rawQuery) ||
			ui.MatchLine(r.Matched, rawQuery) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
