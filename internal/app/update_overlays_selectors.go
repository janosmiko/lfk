package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleNamespaceOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.nsFilterMode {
		return m.handleNamespaceFilterMode(msg)
	}
	return m.handleNamespaceNormalMode(msg)
}

func (m Model) applyPendingUnionSetNamespace(item model.Item) (tea.Model, tea.Cmd) {
	if m.pendingUnionSetName == "" {
		return m, nil
	}
	if item.Name == "" || item.Status == "all" {
		m.setStatusMessage("Union set requires one namespace", true)
		return m, scheduleStatusClear()
	}
	setName := m.pendingUnionSetName
	set, ok := m.findUnionSetConfig(setName)
	if !ok {
		m.pendingUnionSetName = ""
		m.overlay = overlayNone
		m.setStatusMessage(fmt.Sprintf("Union set not found: %s", setName), true)
		return m, scheduleStatusClear()
	}
	m.overlayFilter.Clear()
	m.nsFilterMode = false
	m.overlay = overlayNone
	return m.activateUnionSet(set, item.Name)
}

func (m Model) rejectPendingUnionSetNamespaceMultiSelect() (tea.Model, tea.Cmd) {
	m.setStatusMessage("Union sets use exactly one namespace", true)
	return m, scheduleStatusClear()
}

//nolint:gocyclo // switch-based key dispatch is inherently high-complexity
func (m Model) handleNamespaceNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.filteredOverlayItems()

	switch msg.String() {
	case "esc", "q":
		if m.overlayFilter.Value != "" {
			m.overlayFilter.Clear()
			m.overlayCursor = 0
			return m, nil
		}
		m.overlayFilter.Clear()
		m.pendingUnionSetName = ""
		// Restore the parent overlay (e.g. RBAC) when the namespace
		// selector was opened from inside it; otherwise close fully.
		if m.previousOverlay != overlayNone {
			m.overlay = m.previousOverlay
			m.previousOverlay = overlayNone
			return m, nil
		}
		m.overlay = overlayNone
		return m, nil

	case "enter":
		if m.pendingUnionSetName != "" {
			if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
				return m.applyPendingUnionSetNamespace(items[m.overlayCursor])
			}
			m.setStatusMessage("Union set requires one namespace", true)
			return m, scheduleStatusClear()
		}
		if m.unionMode && (m.overlayCursor < 0 || m.overlayCursor >= len(items) || items[m.overlayCursor].Status == "all") {
			m.setStatusMessage("Union mode supports exactly one namespace", true)
			return m, scheduleStatusClear()
		}
		// Apply selection and close.
		oldNs := m.namespace
		switch {
		case m.nsSelectionModified && len(m.selectedNamespaces) > 0:
			// User explicitly toggled selections with Space in this session.
			m.allNamespaces = false
			if len(m.selectedNamespaces) == 1 {
				for ns := range m.selectedNamespaces {
					m.namespace = ns
				}
			}
		case m.overlayCursor >= 0 && m.overlayCursor < len(items) && items[m.overlayCursor].Status != "all":
			// No Space toggling — apply the cursor position as single namespace.
			ns := items[m.overlayCursor].Name
			m.selectedNamespaces = map[string]bool{ns: true}
			m.namespace = ns
			m.allNamespaces = false
		default:
			// Cursor on "All Namespaces" or no specific item.
			m.selectedNamespaces = nil
			m.allNamespaces = true
		}
		m.invalidateOrphanCacheForNamespace(m.nav.Context, oldNs)
		m.overlayFilter.Clear()
		m.nsFilterMode = false
		m.saveCurrentSession()
		m.cancelAndReset()
		m.requestGen++

		// Restore the parent overlay (e.g. RBAC) when the namespace
		// selector was opened from inside it. The namespace change is
		// global — refreshCurrentLevel runs in BOTH the nested and
		// non-nested cases so the explorer behind the overlay reflects
		// the new scope as soon as the user closes back to it.
		refresh := m.refreshCurrentLevel()
		if m.previousOverlay != overlayNone {
			parent := m.previousOverlay
			m.overlay = parent
			m.previousOverlay = overlayNone
			if parent == overlayCanI {
				m.syncCanINamespacesFromSelection()
				cmds := []tea.Cmd{refresh, m.loadCanIRules()}
				if m.canIMode == canIModeWhoCan && m.whoCan.resource != "" {
					cmds = append(cmds, m.loadWhoCan())
				}
				return m, tea.Batch(cmds...)
			}
			return m, refresh
		}

		m.overlay = overlayNone
		return m, refresh

	case " ":
		if m.pendingUnionSetName != "" {
			return m.rejectPendingUnionSetNamespaceMultiSelect()
		}
		if m.unionMode {
			m.setStatusMessage("Union mode supports exactly one namespace", true)
			return m, scheduleStatusClear()
		}
		m.nsSelectionModified = true
		// Toggle selection on current item.
		if m.overlayCursor >= 0 && m.overlayCursor < len(items) {
			selected := items[m.overlayCursor]
			if selected.Status == "all" {
				// "All Namespaces" selected — clear individual selections.
				m.selectedNamespaces = nil
				m.allNamespaces = true
			} else {
				// Individual namespace — toggle it.
				if m.selectedNamespaces == nil {
					m.selectedNamespaces = make(map[string]bool)
				}
				if m.selectedNamespaces[selected.Name] {
					delete(m.selectedNamespaces, selected.Name)
					if len(m.selectedNamespaces) == 0 {
						m.selectedNamespaces = nil
						m.allNamespaces = true
					}
				} else {
					m.selectedNamespaces[selected.Name] = true
					m.allNamespaces = false
				}
			}
		}
		// Advance cursor to the next item after toggling.
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil

	case ui.ActiveKeybindings.AllNamespaces:
		if m.pendingUnionSetName != "" {
			return m.rejectPendingUnionSetNamespaceMultiSelect()
		}
		if m.unionMode {
			m.setStatusMessage("Union mode supports exactly one namespace", true)
			return m, scheduleStatusClear()
		}
		// Same key the user already uses outside the overlay to flip to
		// all-namespaces mode (default "A"). Drops individual selections
		// and enables all-ns. Cursor jumps to the "All Namespaces" row
		// so a follow-up Enter falls into the default branch (apply
		// all-ns) instead of the "cursor on a real namespace → apply
		// that single namespace" branch, which would silently undo what
		// the user just asked for.
		m.nsSelectionModified = true
		m.selectedNamespaces = nil
		m.allNamespaces = true
		for i, item := range items {
			if item.Status == "all" {
				m.overlayCursor = i
				break
			}
		}
		return m, nil

	case "/":
		m.nsFilterMode = true
		// Snapshot the item under the cursor so Esc can restore it
		// regardless of what the user navigated to in the filtered view.
		m.nsFilterEntryItem = ""
		if items := m.filteredOverlayItems(); m.overlayCursor < len(items) {
			m.nsFilterEntryItem = items[m.overlayCursor].Name
		}
		m.overlayFilter.Clear()
		return m, nil

	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, len(items)-1)
		return m, nil

	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, len(items)-1)
		return m, nil

	case "ctrl+d":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, len(items)-1)
		return m, nil

	case "ctrl+u":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, len(items)-1)
		return m, nil

	case "ctrl+f", "pgdown":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 20, len(items)-1)
		return m, nil

	case "ctrl+b", "pgup":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -20, len(items)-1)
		return m, nil

	case "g":
		if m.pendingG {
			m.pendingG = false
			m.overlayCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G", "end":
		if len(items) > 0 {
			m.overlayCursor = len(items) - 1
		}
		return m, nil

	case "home":
		m.pendingG = false
		m.overlayCursor = 0
		return m, nil

	case "ctrl+c":
		// Close the overlay rather than the tab — once an overlay is
		// open Ctrl+C should match Esc semantics. The tab-close only
		// applies at the explorer level (handleExplorerKey).
		m.overlayFilter.Clear()
		if m.previousOverlay != overlayNone {
			m.overlay = m.previousOverlay
			m.previousOverlay = overlayNone
			return m, nil
		}
		m.overlay = overlayNone
		return m, nil
	}
	return m, nil
}

func (m Model) handleNamespaceFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		switch handlePastedText(&m.overlayFilter, msg.Runes) {
		case filterContinue:
			m.overlayCursor = 0
			return m, nil
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), pasteTargetOverlayFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.overlayFilter, msg.String()) {
	case filterEscape:
		// Restore the cursor to the item that was selected when filter
		// mode was entered — not whatever the user navigated to in the
		// filtered view. The snapshot was taken in the "/" case handler.
		target := m.nsFilterEntryItem
		m.nsFilterMode = false
		m.nsFilterEntryItem = ""
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		if target != "" {
			for i, it := range m.filteredOverlayItems() {
				if it.Name == target {
					m.overlayCursor = i
					break
				}
			}
		}
		return m, nil
	case filterAccept:
		m.nsFilterMode = false
		m.overlayCursor = 0
		if m.pendingUnionSetName != "" {
			items := m.filteredOverlayItems()
			if len(items) == 1 {
				return m.applyPendingUnionSetNamespace(items[0])
			}
			return m, nil
		}
		// When the filter narrows to a single result and the user hasn't
		// been multi-selecting with Space, Enter is unambiguous: apply
		// that result and close. Without this, the user has to press
		// Enter twice (once to leave filter mode, once to commit) on a
		// list that has only one row — the second Enter is busy work.
		if !m.nsSelectionModified {
			items := m.filteredOverlayItems()
			if len(items) == 1 {
				oldNs := m.namespace
				if items[0].Status == "all" {
					m.selectedNamespaces = nil
					m.allNamespaces = true
				} else {
					ns := items[0].Name
					m.selectedNamespaces = map[string]bool{ns: true}
					m.namespace = ns
					m.allNamespaces = false
				}
				m.invalidateOrphanCacheForNamespace(m.nav.Context, oldNs)
				m.overlay = overlayNone
				m.overlayFilter.Clear()
				m.saveCurrentSession()
				m.cancelAndReset()
				m.requestGen++
				return m, m.refreshCurrentLevel()
			}
		}
		return m, nil
	case filterClose:
		// Ctrl+C in filter mode closes the overlay, not the tab —
		// matches the Esc/q behaviour in the surrounding normal-mode
		// handler so users never accidentally drop a tab while searching.
		m.nsFilterMode = false
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		if m.previousOverlay != overlayNone {
			m.overlay = m.previousOverlay
			m.previousOverlay = overlayNone
			return m, nil
		}
		m.overlay = overlayNone
		return m, nil
	case filterContinue:
		m.overlayCursor = 0
		return m, nil
	}
	return m, nil
}

func (m Model) handleTemplateOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.templateSearchMode {
		return m.handleTemplateFilterMode(msg)
	}

	filtered := m.filteredTemplates()

	switch msg.String() {
	case "esc", "q":
		// If filter is active, first esc clears filter; second closes overlay.
		if m.templateFilter.Value != "" {
			m.templateFilter.Clear()
			m.templateCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		return m, nil
	case "enter":
		if len(filtered) > 0 && m.templateCursor >= 0 && m.templateCursor < len(filtered) {
			tmpl := filtered[m.templateCursor]
			m.overlay = overlayNone
			m.templateFilter.Clear()
			return m, m.applyTemplate(tmpl)
		}
		return m, nil
	case "up", "k", "ctrl+p":
		m.templateCursor = clampOverlayCursor(m.templateCursor, -1, len(filtered)-1)
		return m, nil
	case "down", "j", "ctrl+n":
		m.templateCursor = clampOverlayCursor(m.templateCursor, 1, len(filtered)-1)
		return m, nil
	case "ctrl+d":
		m.templateCursor = clampOverlayCursor(m.templateCursor, 10, len(filtered)-1)
		return m, nil
	case "ctrl+u":
		m.templateCursor = clampOverlayCursor(m.templateCursor, -10, len(filtered)-1)
		return m, nil
	case "ctrl+f", "pgdown":
		m.templateCursor = clampOverlayCursor(m.templateCursor, 20, len(filtered)-1)
		return m, nil
	case "ctrl+b", "pgup":
		m.templateCursor = clampOverlayCursor(m.templateCursor, -20, len(filtered)-1)
		return m, nil
	case "home":
		m.pendingG = false
		m.templateCursor = 0
		return m, nil
	case "end":
		m.pendingG = false
		if len(filtered) > 0 {
			m.templateCursor = len(filtered) - 1
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.templateCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		if len(filtered) > 0 {
			m.templateCursor = len(filtered) - 1
		}
		return m, nil
	case "/":
		m.templateSearchMode = true
		m.templateFilter.Clear()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleTemplateFilterMode handles keys when the template overlay is in filter input mode.
func (m Model) handleTemplateFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		switch handlePastedText(&m.templateFilter, msg.Runes) {
		case filterContinue:
			m.templateCursor = 0
			return m, nil
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), pasteTargetTemplateFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.templateFilter, msg.String()) {
	case filterEscape:
		// Preserve cursor on the same template across filter exit.
		var targetName string
		if items := m.filteredTemplates(); m.templateCursor < len(items) {
			targetName = items[m.templateCursor].Name
		}
		m.templateSearchMode = false
		m.templateFilter.Clear()
		m.templateCursor = 0
		if targetName != "" {
			for i, t := range m.filteredTemplates() {
				if t.Name == targetName {
					m.templateCursor = i
					break
				}
			}
		}
		return m, nil
	case filterAccept:
		m.templateSearchMode = false
		// When the filter narrows to a single template, Enter is unambiguous:
		// apply it and close. Without this, the user has to press Enter twice
		// (once to leave filter mode, once to commit) on a one-row list.
		filtered := m.filteredTemplates()
		if len(filtered) == 1 {
			tmpl := filtered[0]
			m.overlay = overlayNone
			m.templateFilter.Clear()
			return m, m.applyTemplate(tmpl)
		}
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
	case filterContinue:
		m.templateCursor = 0
		return m, nil
	}
	return m, nil
}

// filteredTemplates returns templates matching the current template filter.
// Matches against Name, Description, and Category using the shared search utility.
func (m *Model) filteredTemplates() []model.ResourceTemplate {
	if m.templateFilter.Value == "" {
		return m.templateItems
	}
	rawQuery := m.templateFilter.Value
	var filtered []model.ResourceTemplate
	for _, tmpl := range m.templateItems {
		if ui.MatchLine(tmpl.Name, rawQuery) ||
			ui.MatchLine(tmpl.Description, rawQuery) ||
			ui.MatchLine(tmpl.Category, rawQuery) {
			filtered = append(filtered, tmpl)
		}
	}
	return filtered
}
