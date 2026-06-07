package app

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

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
		// Re-render cached previews so they drop the live-preview theme's baked
		// colors and paint with the restored theme immediately.
		m = m.recomposeThemedContent()
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
			// Re-render cached previews so the new theme applies immediately
			// instead of waiting for the next data tick.
			m = m.recomposeThemedContent()
			return m, scheduleStatusClear()
		}
		return m, nil

	case "/":
		m.schemeFilterMode = true
		// Snapshot pre-filter cursor target so Esc restores it.
		m.schemeFilterEntryName = ""
		if names := m.filteredSchemeNames(); m.schemeCursor < len(names) {
			m.schemeFilterEntryName = names[m.schemeCursor]
		}
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

	case "ctrl+d", "shift+down":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, 10, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+u", "shift+up":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, -10, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+f", "pgdown":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, 20, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "ctrl+b", "pgup":
		m.schemeCursor = clampOverlayCursor(m.schemeCursor, -20, selectableCount-1)
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "home":
		m.pendingG = false
		m.schemeCursor = 0
		ui.ResetOverlaySchemeScroll()
		m.previewSchemeAtCursor(filtered)
		return m, nil

	case "end":
		m.pendingG = false
		if selectableCount > 0 {
			m.schemeCursor = selectableCount - 1
		}
		ui.ResetOverlaySchemeScroll()
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
		// Re-render cached previews so the transparency change applies now.
		m = m.recomposeThemedContent()
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
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), pasteTargetSchemeFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.schemeFilter, msg.String()) {
	case filterEscape:
		// Restore the cursor to the scheme that was selected when filter
		// mode was entered. The snapshot was taken in the "/" case.
		target := m.schemeFilterEntryName
		m.schemeFilterMode = false
		m.schemeFilterEntryName = ""
		m.schemeFilter.Clear()
		m.schemeCursor = 0
		ui.ResetOverlaySchemeScroll()
		if target != "" {
			for i, n := range m.filteredSchemeNames() {
				if n == target {
					m.schemeCursor = i
					break
				}
			}
		}
		m.previewSchemeAtCursor(m.filteredSchemeNames())
		return m, nil
	case filterAccept:
		m.schemeFilterMode = false
		m.schemeCursor = 0
		ui.ResetOverlaySchemeScroll()
		filtered := m.filteredSchemeNames()
		// When the filter narrows to a single scheme, Enter is unambiguous:
		// commit it and close. The live preview already applied the theme on
		// each keystroke; here we lock it in with a status message so the
		// user does not have to press Enter again on a one-row list.
		if len(filtered) == 1 {
			name := filtered[0]
			schemes := ui.BuiltinSchemes()
			if theme, ok := schemes[name]; ok {
				ui.ApplyTheme(theme)
				ui.ActiveSchemeName = name
				m.setStatusMessage("Color scheme: "+name, false)
			}
			m.overlay = overlayNone
			m.schemeFilter.Clear()
			// Re-render cached previews so the new theme applies immediately.
			m = m.recomposeThemedContent()
			return m, scheduleStatusClear()
		}
		m.previewSchemeAtCursor(filtered)
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
// The colorscheme picker keeps the explorer behind it un-dimmed for side-by-side
// comparison (see renderOverlay), so the cached preview strings (dashboard,
// monitoring, metrics bar, events footer) must be recomposed on every preview
// step — otherwise their baked-in theme colors stay stale until the next tick.
func (m *Model) previewSchemeAtCursor(filtered []string) {
	if m.schemeCursor >= 0 && m.schemeCursor < len(filtered) {
		name := filtered[m.schemeCursor]
		schemes := ui.BuiltinSchemes()
		if theme, ok := schemes[name]; ok {
			ui.ApplyTheme(theme)
			ui.ActiveSchemeName = name
			*m = m.recomposeThemedContent()
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
	end := min(start+ui.GetOverlaySchemeVisible(), len(items))
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
	end := min(start+ui.GetOverlaySchemeVisible(), len(items))
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
