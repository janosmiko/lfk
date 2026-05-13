package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/ui"
)

// handleFinalizerSearchKey handles keyboard input for the finalizer search overlay.
func (m Model) handleFinalizerSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// When filter input is active, handle text input first.
	if m.finalizerSearchFilterActive {
		return m.handleFinalizerSearchFilterKey(msg)
	}

	filtered := m.filteredFinalizerResults()
	maxIdx := len(filtered) - 1

	switch key {
	case "esc", "q":
		m.overlay = overlayNone
		m.finalizerSearchResults = nil
		m.finalizerSearchSelected = nil
		m.finalizerSearchFilter = ""
		m.finalizerSearchFilterActive = false
		return m, nil

	case "j", "down":
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, 1, maxIdx)
		return m, nil

	case "k", "up":
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, -1, maxIdx)
		return m, nil

	case "g":
		// gg to top.
		if m.pendingG {
			m.pendingG = false
			m.finalizerSearchCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G":
		m.finalizerSearchCursor = maxIdx
		return m, nil

	case "ctrl+d":
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, 10, maxIdx)
		return m, nil

	case "ctrl+u":
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, -10, maxIdx)
		return m, nil

	case "ctrl+f":
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, 20, maxIdx)
		return m, nil

	case "ctrl+b":
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, -20, maxIdx)
		return m, nil

	case " ":
		// Toggle selection on the current item and advance cursor.
		if m.finalizerSearchCursor >= 0 && m.finalizerSearchCursor < len(filtered) {
			match := filtered[m.finalizerSearchCursor]
			k := finalizerMatchKey(match)
			if m.finalizerSearchSelected[k] {
				delete(m.finalizerSearchSelected, k)
			} else {
				m.finalizerSearchSelected[k] = true
			}
		}
		m.finalizerSearchCursor = clampOverlayCursor(m.finalizerSearchCursor, 1, maxIdx)
		return m, nil

	case "ctrl+a":
		// Select/deselect all visible (filtered) results.
		allSelected := true
		for _, match := range filtered {
			if !m.finalizerSearchSelected[finalizerMatchKey(match)] {
				allSelected = false
				break
			}
		}
		if allSelected {
			// Deselect all.
			for _, match := range filtered {
				delete(m.finalizerSearchSelected, finalizerMatchKey(match))
			}
		} else {
			// Select all.
			for _, match := range filtered {
				m.finalizerSearchSelected[finalizerMatchKey(match)] = true
			}
		}
		return m, nil

	case "enter":
		// Confirm removal: open a type-to-confirm overlay.
		selectedCount := len(m.finalizerSearchSelected)
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
		m.finalizerSearchFilterActive = true
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
	}

	return m, nil
}

// handleFinalizerSearchFilterKey handles keyboard input when the filter bar
// is active inside the finalizer search overlay.
func (m Model) handleFinalizerSearchFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Pasted text arrives as a single key event with Paste=true and the
	// pasted content in Runes. Append single-line pastes to the filter;
	// drop multi-line pastes (the finalizer filter is single-line by
	// design, so collapsing newlines would silently hide content).
	if msg.Paste {
		pasted := strings.TrimRight(string(msg.Runes), "\n")
		if !strings.ContainsAny(pasted, "\n\r") {
			m.finalizerSearchFilter += pasted
			m.finalizerSearchCursor = 0
		}
		return m, nil
	}
	key := msg.String()
	switch key {
	case "esc":
		if m.finalizerSearchFilter != "" {
			m.finalizerSearchFilter = ""
			m.finalizerSearchCursor = 0
		} else if m.finalizerSearchResults == nil && m.finalizerSearchPattern == "" {
			// No search performed yet — close the overlay entirely.
			m.finalizerSearchFilterActive = false
			m.overlay = overlayNone
		} else {
			m.finalizerSearchFilterActive = false
		}
		return m, nil
	case "enter":
		if m.finalizerSearchResults == nil && m.finalizerSearchPattern == "" {
			// Initial search prompt: use filter text as the search pattern.
			pattern := strings.TrimSpace(m.finalizerSearchFilter)
			if pattern == "" {
				return m, nil
			}
			m.finalizerSearchPattern = pattern
			m.finalizerSearchFilter = ""
			m.finalizerSearchFilterActive = false
			m.finalizerSearchLoading = true
			return m, m.searchFinalizers(pattern)
		}
		m.finalizerSearchFilterActive = false
		return m, nil
	case "backspace":
		if len(m.finalizerSearchFilter) > 0 {
			m.finalizerSearchFilter = m.finalizerSearchFilter[:len(m.finalizerSearchFilter)-1]
			m.finalizerSearchCursor = 0
		}
		return m, nil
	case "ctrl+w":
		// Delete word.
		f := m.finalizerSearchFilter
		f = strings.TrimRight(f, " ")
		if idx := strings.LastIndex(f, " "); idx >= 0 {
			m.finalizerSearchFilter = f[:idx+1]
		} else {
			m.finalizerSearchFilter = ""
		}
		m.finalizerSearchCursor = 0
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.finalizerSearchFilter += key
			m.finalizerSearchCursor = 0
		}
		return m, nil
	}
}

// filteredFinalizerResults returns the finalizer search results filtered
// by the current filter text (matching name, namespace, or kind).
func (m Model) filteredFinalizerResults() []k8s.FinalizerMatch {
	if m.finalizerSearchFilter == "" {
		return m.finalizerSearchResults
	}
	rawQuery := m.finalizerSearchFilter
	var filtered []k8s.FinalizerMatch
	for _, r := range m.finalizerSearchResults {
		if ui.MatchLine(r.Name, rawQuery) ||
			ui.MatchLine(r.Namespace, rawQuery) ||
			ui.MatchLine(r.Kind, rawQuery) ||
			ui.MatchLine(r.Matched, rawQuery) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
