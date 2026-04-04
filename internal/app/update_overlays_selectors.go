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

func (m Model) handleNamespaceNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	items := m.filteredOverlayItems()

	switch msg.String() {
	case "esc", "q":
		if m.overlayFilter.Value != "" {
			m.overlayFilter.Clear()
			m.overlayCursor = 0
			return m, nil
		}
		m.overlay = overlayNone
		m.overlayFilter.Clear()
		return m, nil

	case "enter":
		// Apply selection and close.
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
		m.overlay = overlayNone
		m.overlayFilter.Clear()
		m.nsFilterMode = false
		m.saveCurrentSession()
		m.cancelAndReset()
		m.requestGen++
		return m, m.refreshCurrentLevel()

	case " ":
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

	case "c":
		m.nsSelectionModified = true
		// Clear all namespace selections (reset to all namespaces).
		m.selectedNamespaces = nil
		m.allNamespaces = true
		return m, nil

	case "/":
		m.nsFilterMode = true
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

	case "ctrl+f":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 20, len(items)-1)
		return m, nil

	case "ctrl+b":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -20, len(items)-1)
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()
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
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), &m.overlayFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.overlayFilter, msg.String()) {
	case filterEscape:
		m.nsFilterMode = false
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.nsFilterMode = false
		m.overlayCursor = 0
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
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
	case "ctrl+f":
		m.templateCursor = clampOverlayCursor(m.templateCursor, 20, len(filtered)-1)
		return m, nil
	case "ctrl+b":
		m.templateCursor = clampOverlayCursor(m.templateCursor, -20, len(filtered)-1)
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
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), &m.templateFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.templateFilter, msg.String()) {
	case filterEscape:
		m.templateSearchMode = false
		m.templateFilter.Clear()
		m.templateCursor = 0
		return m, nil
	case filterAccept:
		m.templateSearchMode = false
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

func (m Model) handleRollbackOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.rollbackRevisions = nil
		return m, nil
	case "j", "down":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, 1, len(m.rollbackRevisions)-1)
		return m, nil
	case "k", "up":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, -1, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+d":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, 10, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+u":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, -10, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+f":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, 20, len(m.rollbackRevisions)-1)
		return m, nil
	case "ctrl+b":
		m.rollbackCursor = clampOverlayCursor(m.rollbackCursor, -20, len(m.rollbackRevisions)-1)
		return m, nil
	case "enter":
		if m.rollbackCursor >= 0 && m.rollbackCursor < len(m.rollbackRevisions) {
			rev := m.rollbackRevisions[m.rollbackCursor]
			m.addLogEntry("DBG", fmt.Sprintf("Rolling back to revision %d (RS: %s)", rev.Revision, rev.Name))
			m.loading = true
			return m, m.rollbackDeployment(rev.Revision)
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleHelmRollbackOverlayKey handles keyboard input for the Helm rollback overlay.
func (m Model) handleHelmRollbackOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.overlay = overlayNone
		m.helmRollbackRevisions = nil
		return m, nil
	case "j", "down":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, 1, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "k", "up":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, -1, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+d":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, 10, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+u":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, -10, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+f":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, 20, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "ctrl+b":
		m.helmRollbackCursor = clampOverlayCursor(m.helmRollbackCursor, -20, len(m.helmRollbackRevisions)-1)
		return m, nil
	case "enter":
		if m.helmRollbackCursor >= 0 && m.helmRollbackCursor < len(m.helmRollbackRevisions) {
			rev := m.helmRollbackRevisions[m.helmRollbackCursor]
			m.addLogEntry("DBG", fmt.Sprintf("Rolling back Helm release to revision %d", rev.Revision))
			m.loading = true
			return m, m.rollbackHelmRelease(rev.Revision)
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleColorschemeOverlayKey handles keyboard input for the color scheme selector overlay.
func (m Model) handleColorschemeOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.schemeFilterMode {
		return m.handleColorschemeFilterMode(msg)
	}
	return m.handleColorschemeNormalMode(msg)
}

func (m Model) handleColorschemeNormalMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredSchemeNames()
	selectableCount := len(filtered)

	switch msg.String() {
	case "esc", "q":
		if m.schemeFilter.Value != "" {
			m.schemeFilter.Clear()
			m.schemeCursor = 0
			m.previewSchemeAtCursor(m.filteredSchemeNames())
			return m, nil
		}
		// Restore original theme on cancel.
		schemes := ui.BuiltinSchemes()
		if theme, ok := schemes[m.schemeOriginalName]; ok {
			ui.ApplyTheme(theme)
			ui.ActiveSchemeName = m.schemeOriginalName
		}
		m.overlay = overlayNone
		m.schemeFilter.Clear()
		return m, nil

	case "enter":
		if selectableCount > 0 && m.schemeCursor >= 0 && m.schemeCursor < selectableCount {
			name := filtered[m.schemeCursor]
			schemes := ui.BuiltinSchemes()
			if theme, ok := schemes[name]; ok {
				ui.ApplyTheme(theme)
				ui.ActiveSchemeName = name
				m.setStatusMessage("Color scheme: "+name, false)
			}
			m.overlay = overlayNone
			m.schemeFilter.Clear()
			return m, scheduleStatusClear()
		}
		return m, nil

	case "/":
		m.schemeFilterMode = true
		m.schemeFilter.Clear()
		return m, nil

	case "j", "down", "ctrl+n":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, 1, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "k", "up", "ctrl+p":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, -1, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+d":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, 10, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+u":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, -10, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+f":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, 20, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+b":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, -20, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "g":
		if m.pendingG {
			m.pendingG = false
			m.schemeCursor = 0
			ui.ResetOverlaySchemeScroll()
			m.previewSchemeAtCursor(filtered)
			return m, nil
		}
		m.pendingG = true
		return m, nil

	case "G":
		if selectableCount > 0 {
			m.schemeCursor = selectableCount - 1
		}
		ui.ResetOverlaySchemeScroll()
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "H":
		// Jump to the first visible selectable item.
		m.schemeCursor = m.schemeFirstVisibleSelectable()
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "L":
		// Jump to the last visible selectable item.
		m.schemeCursor = m.schemeLastVisibleSelectable()
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "t":
		ui.ConfigTransparentBg = !ui.ConfigTransparentBg
		// Re-apply current theme to update bar styles.
		if theme, ok := ui.BuiltinSchemes()[ui.ActiveSchemeName]; ok {
			ui.ApplyTheme(theme)
		}
		if ui.ConfigTransparentBg {
			m.setStatusMessage("Transparent background: on", false)
		} else {
			m.setStatusMessage("Transparent background: off", false)
		}
		return m, scheduleStatusClear()

	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

func (m Model) handleColorschemeFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		switch handlePastedText(&m.schemeFilter, msg.Runes) {
		case filterContinue:
			m.schemeCursor = 0
			ui.ResetOverlaySchemeScroll()
			m.previewSchemeAtCursor(m.filteredSchemeNames())
			return m, nil
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), &m.schemeFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.schemeFilter, msg.String()) {
	case filterEscape:
		m.schemeFilterMode = false
		m.schemeFilter.Clear()
		m.schemeCursor = 0
		ui.ResetOverlaySchemeScroll()
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	case filterAccept:
		m.schemeFilterMode = false
		m.schemeCursor = 0
		ui.ResetOverlaySchemeScroll()
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
	case filterContinue:
		m.schemeCursor = 0
		ui.ResetOverlaySchemeScroll()
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	}
	return m, nil
}

// previewSchemeAtCursor applies the scheme under the cursor as a live preview.
func (m *Model) previewSchemeAtCursor(filtered []string) {
	if m.schemeCursor >= 0 && m.schemeCursor < len(filtered) {
		name := filtered[m.schemeCursor]
		schemes := ui.BuiltinSchemes()
		if theme, ok := schemes[name]; ok {
			ui.ApplyTheme(theme)
			ui.ActiveSchemeName = name
		}
	}
}

// filteredSchemeNames returns the selectable scheme names filtered by the current filter text.
func (m *Model) filteredSchemeNames() []string {
	var result []string
	if m.schemeFilter.Value == "" {
		for _, e := range m.schemeEntries {
			if !e.IsHeader {
				result = append(result, e.Name)
			}
		}
		return result
	}
	lower := strings.ToLower(m.schemeFilter.Value)
	for _, e := range m.schemeEntries {
		if e.IsHeader {
			continue
		}
		if strings.Contains(e.Name, lower) {
			result = append(result, e.Name)
		}
	}
	return result
}

// schemeFirstVisibleSelectable returns the selectIdx of the first selectable
// item currently visible in the colorscheme overlay viewport.
func (m *Model) schemeFirstVisibleSelectable() int {
	items := m.schemeDisplayItems()
	start := ui.GetOverlaySchemeScroll()
	end := start + ui.SchemeOverlayMaxVisible
	if end > len(items) {
		end = len(items)
	}
	for i := start; i < end; i++ {
		if items[i].selectIdx >= 0 {
			return items[i].selectIdx
		}
	}
	return m.schemeCursor
}

// schemeLastVisibleSelectable returns the selectIdx of the last selectable
// item currently visible in the colorscheme overlay viewport.
func (m *Model) schemeLastVisibleSelectable() int {
	items := m.schemeDisplayItems()
	start := ui.GetOverlaySchemeScroll()
	end := start + ui.SchemeOverlayMaxVisible
	if end > len(items) {
		end = len(items)
	}
	for i := end - 1; i >= start; i-- {
		if items[i].selectIdx >= 0 {
			return items[i].selectIdx
		}
	}
	return m.schemeCursor
}

// schemeDisplayItem mirrors the display list structure from RenderColorschemeOverlay.
type schemeDisplayItem struct {
	selectIdx int // -1 for headers
}

// schemeDisplayItems builds the display list matching RenderColorschemeOverlay's logic.
func (m *Model) schemeDisplayItems() []schemeDisplayItem {
	var items []schemeDisplayItem
	selectIdx := 0
	if m.schemeFilter.Value == "" {
		for _, e := range m.schemeEntries {
			if e.IsHeader {
				items = append(items, schemeDisplayItem{selectIdx: -1})
			} else {
				items = append(items, schemeDisplayItem{selectIdx: selectIdx})
				selectIdx++
			}
		}
	} else {
		lower := strings.ToLower(m.schemeFilter.Value)
		for _, e := range m.schemeEntries {
			if e.IsHeader {
				continue
			}
			if strings.Contains(e.Name, lower) {
				items = append(items, schemeDisplayItem{selectIdx: selectIdx})
				selectIdx++
			}
		}
	}
	return items
}
