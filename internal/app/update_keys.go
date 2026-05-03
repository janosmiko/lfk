package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Dismiss startup tip on any keypress.
	if m.statusMessageTip {
		m.statusMessage = ""
		m.statusMessageTip = false
	}

	// Handle regular overlays first so when an overlay (e.g. the theme
	// selector) is opened on top of the error log, its own keys —
	// including j/k navigation and Esc — reach handleOverlayKey instead
	// of being eaten by the error log handler.
	if m.overlay != overlayNone {
		return m.handleOverlayKey(msg)
	}

	// Handle error log overlay (independent of regular overlays).
	if m.overlayErrorLog {
		return m.handleErrorLogOverlayKey(msg)
	}

	// Handle command bar input mode.
	if m.commandBarActive {
		return m.handleCommandBarKey(msg)
	}

	// Handle filter input mode.
	if m.filterActive {
		return m.handleFilterKey(msg)
	}

	// Handle search input mode.
	if m.searchActive {
		return m.handleSearchKey(msg)
	}

	// Tab switching keys work in all fullscreen modes (YAML, Logs, Describe, Diff, Help)
	// as long as the user is not in a text input sub-mode (search, etc.).
	if mdl, cmd, handled := m.handleTabSwitchKey(msg); handled {
		return mdl, cmd
	}

	// Dispatch to mode-specific handlers.
	if mdl, cmd, handled := m.handleModeKey(msg); handled {
		return mdl, cmd
	}

	// Explorer mode key handling.
	return m.handleExplorerKey(msg)
}

// handleTabSwitchKey handles tab switching keys (next/prev/new tab).
func (m Model) handleTabSwitchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	if m.mode == modeExplorer || m.mode == modeExec || m.yamlSearchMode || m.logSearchActive || m.helpSearchActive || m.explainSearchActive || m.diffSearchMode {
		return m, nil, false
	}
	switch msg.String() {
	case kb.NextTab:
		if len(m.tabs) > 1 {
			// Auto-pause Kubetris when switching tabs.
			if m.mode == modeKubetris && m.kubetrisGame != nil && !m.kubetrisGame.paused {
				m.kubetrisGame.paused = true
			}
			m.saveCurrentTab()
			next := (m.activeTab + 1) % len(m.tabs)
			if cmd := m.loadTab(next); cmd != nil {
				return m, cmd, true
			}
			return m, m.postTabSwitchCmd(), true
		}
	case kb.PrevTab:
		if len(m.tabs) > 1 {
			if m.mode == modeKubetris && m.kubetrisGame != nil && !m.kubetrisGame.paused {
				m.kubetrisGame.paused = true
			}
			m.saveCurrentTab()
			prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			if cmd := m.loadTab(prev); cmd != nil {
				return m, cmd, true
			}
			return m, m.postTabSwitchCmd(), true
		}
	case kb.NewTab:
		if m.mode == modeKubetris && m.kubetrisGame != nil && !m.kubetrisGame.paused {
			m.kubetrisGame.paused = true
		}
		if m.mode != modeHelp {
			if len(m.tabs) >= 9 {
				m.setStatusMessage("Max 9 tabs", true)
				return m, scheduleStatusClear(), true
			}
			m.saveCurrentTab()
			newTab := m.cloneCurrentTab()
			newTab.mode = modeExplorer
			newTab.logLines = nil
			newTab.logCancel = nil
			newTab.logCh = nil
			insertAt := m.activeTab + 1
			m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
			m.activeTab = insertAt
			m.loadTab(m.activeTab)
			m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
			return m, scheduleStatusClear(), true
		}
	}
	return m, nil, false
}

// postTabSwitchCmd returns the appropriate command after switching tabs.
func (m Model) postTabSwitchCmd() tea.Cmd {
	if m.mode == modeExplorer {
		return m.loadPreview()
	}
	if m.mode == modeLogs && m.logCh != nil {
		return m.waitForLogLine()
	}
	if m.mode == modeExec && m.execPTY != nil {
		return m.scheduleExecTick()
	}
	return nil
}

// handleModeKey dispatches to the appropriate mode-specific handler.
func (m Model) handleModeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch m.mode {
	case modeExec:
		mdl, cmd := m.handleExecKey(msg)
		return mdl, cmd, true
	case modeHelp:
		mdl, cmd := m.handleHelpKey(msg)
		return mdl, cmd, true
	case modeLogs:
		mdl, cmd := m.handleLogKey(msg)
		return mdl, cmd, true
	case modeDiff:
		mdl, cmd := m.handleDiffKey(msg)
		return mdl, cmd, true
	case modeDescribe:
		mdl, cmd := m.handleDescribeKey(msg)
		return mdl, cmd, true
	case modeEventViewer:
		mdl, cmd := m.handleEventViewerModeKey(msg)
		return mdl, cmd, true
	case modeExplain:
		if m.explainSearchActive {
			mdl, cmd := m.handleExplainSearchKey(msg)
			return mdl, cmd, true
		}
		mdl, cmd := m.handleExplainKey(msg)
		return mdl, cmd, true
	case modeYAML:
		mdl, cmd := m.handleYAMLKey(msg)
		return mdl, cmd, true
	case modeKubetris:
		mdl, cmd := m.handleKubetrisKey(msg)
		return mdl, cmd, true
	case modeCredits:
		// Any key exits the credits screen.
		m.mode = modeExplorer
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) handleKeyThemeSelector() Model {
	m.schemeEntries = ui.GroupedSchemeEntries()
	m.schemeCursor = 0
	m.schemeFilter.Clear()
	m.schemeOriginalName = ui.ActiveSchemeName
	ui.ResetOverlaySchemeScroll()
	// Position cursor on the currently active scheme.
	selectIdx := 0
	for _, e := range m.schemeEntries {
		if e.IsHeader {
			continue
		}
		if e.Name == ui.ActiveSchemeName {
			m.schemeCursor = selectIdx
			break
		}
		selectIdx++
	}
	m.overlay = overlayColorscheme
	return m
}

func (m Model) handleKeySelectRange() Model {
	if m.nav.Level < model.LevelResources {
		return m
	}
	items := m.visibleMiddleItems()
	if len(items) == 0 {
		return m
	}
	cur := m.cursor()
	if m.selectionAnchor < 0 {
		// No anchor set — toggle current item and set anchor.
		if sel := m.selectedMiddleItem(); sel != nil {
			m.toggleSelection(*sel)
			m.selectionAnchor = cur
		}
		return m
	}
	// Select range from anchor to cursor (inclusive).
	lo, hi := m.selectionAnchor, cur
	if lo > hi {
		lo, hi = hi, lo
	}
	for i := lo; i <= hi && i < len(items); i++ {
		m.selectedItems[selectionKey(items[i])] = true
	}
	m.selectionRev++
	return m
}

func (m Model) handleKeyToggleSelect() Model {
	if m.nav.Level >= model.LevelResources {
		sel := m.selectedMiddleItem()
		if sel != nil {
			m.toggleSelection(*sel)
			// Set anchor when selecting, reset when deselecting.
			if m.isSelected(*sel) {
				m.selectionAnchor = m.cursor()
			} else {
				m.selectionAnchor = -1
			}
		}
		// Move cursor down.
		visible := m.visibleMiddleItems()
		c := m.cursor() + 1
		if c >= len(visible) {
			c = len(visible) - 1
		}
		if c < 0 {
			c = 0
		}
		m.setCursor(c)
		return m
	}
	return m
}

func (m Model) handleKeySelectAll() Model {
	if m.nav.Level >= model.LevelResources {
		visible := m.visibleMiddleItems()
		if m.hasSelection() {
			// If any are selected, deselect all.
			m.clearSelection()
		} else {
			// Select all visible items.
			for _, item := range visible {
				m.selectedItems[selectionKey(item)] = true
			}
			m.selectionRev++
		}
		m.selectionAnchor = -1
		return m
	}
	return m
}

func (m Model) handleKeyNextMatch() (tea.Model, tea.Cmd) {
	if m.searchInput.Value != "" {
		m.jumpToSearchMatch(m.cursor() + 1)
		m.syncExpandedGroup()
		return m, m.loadPreview()
	}
	return m, nil
}

func (m Model) handleKeyPrevMatch() (tea.Model, tea.Cmd) {
	if m.searchInput.Value != "" {
		m.jumpToPrevSearchMatch(m.cursor() - 1)
		m.syncExpandedGroup()
		return m, m.loadPreview()
	}
	return m, nil
}

func (m Model) handleKeyNamespaceSelector() (tea.Model, tea.Cmd) {
	m.overlay = overlayNamespace
	m.overlayFilter.Clear()
	ui.ResetOverlayNsScroll()
	m.nsSelectionModified = false

	// Reuse the existing per-context namespace cache (also used by the
	// command-bar autocompleter). When a cached entry exists we seed the
	// overlay synchronously so it opens instantly — even when the entry
	// is stale we show its rows immediately and rely on the
	// stale-while-revalidate refresh to swap in fresh data shortly after.
	// Only the empty/missing-cache case still pays the loading-spinner +
	// API round-trip path the original implementation always took.
	entry, ok := m.cachedNamespaces[m.activeContext()]
	if ok && len(entry.items) > 0 {
		m.overlayItems = buildNamespaceOverlayItems(entry.items)
		m.overlayCursor = namespaceOverlayCursor(m.overlayItems, m.namespace, m.allNamespaces)
		m.loading = false
		// ensureNamespaceCacheFresh returns nil when the entry is fresh
		// (cache hit, no work) and a silent loader when it has aged past
		// namespaceCacheTTL — the silent flag keeps the spinner off and
		// makes updateNamespacesLoaded skip the overlay-state rewrite, so
		// the user's cursor and item list survive the background refresh.
		return m, m.ensureNamespaceCacheFresh()
	}

	m.overlayItems = nil // populated when namespacesLoadedMsg arrives
	m.overlayCursor = 0
	m.loading = true
	return m, m.loadNamespaces()
}

func (m Model) handleKeyWatchMode() (tea.Model, tea.Cmd) {
	m.watchMode = !m.watchMode
	if m.watchMode {
		m.setStatusMessage(fmt.Sprintf("Watch mode ON (refresh every %s)", m.watchInterval), false)
		return m, tea.Batch(scheduleWatchTick(m.watchInterval), scheduleStatusClear())
	}
	m.setStatusMessage("Watch mode OFF", false)
	return m, scheduleStatusClear()
}

func (m Model) handleKeyExpandCollapse() (tea.Model, tea.Cmd) {
	// At the Events resource list, reuse the expand/collapse key to toggle
	// grouping of duplicate events (same Type/Reason/Message/Object collapsed
	// into a single row with a summed Count).
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "Event" {
		m.eventGrouping = !m.eventGrouping
		m.rebuildEventsFromCache()
		if m.eventGrouping {
			m.setStatusMessage("Events grouped (duplicates collapsed)", false)
		} else {
			m.setStatusMessage("Events expanded (raw)", false)
		}
		return m, scheduleStatusClear()
	}
	if m.nav.Level == model.LevelResourceTypes {
		if m.allGroupsExpanded {
			// Collapsing: find current item's category BEFORE changing mode.
			visible := m.visibleMiddleItems()
			c := m.cursor()
			if c >= 0 && c < len(visible) {
				m.expandedGroup = visible[c].Category
			}
			m.allGroupsExpanded = false
			// Find the same item in the now-collapsed visible list.
			if c >= 0 && c < len(visible) {
				targetItem := visible[c]
				newVisible := m.visibleMiddleItems()
				for i, item := range newVisible {
					if item.Name == targetItem.Name && item.Kind == targetItem.Kind && item.Category == targetItem.Category {
						m.setCursor(i)
						break
					}
				}
			}
			m.clampCursor()
			m.setStatusMessage("Groups collapsed (accordion mode)", false)
		} else {
			// Expanding: find current item BEFORE changing mode.
			visible := m.visibleMiddleItems()
			c := m.cursor()
			var targetItem model.Item
			if c >= 0 && c < len(visible) {
				targetItem = visible[c]
			}
			m.allGroupsExpanded = true
			// Find the same item in the now-expanded visible list.
			if targetItem.Name != "" {
				newVisible := m.visibleMiddleItems()
				for i, item := range newVisible {
					if item.Name == targetItem.Name && item.Kind == targetItem.Kind && item.Category == targetItem.Category {
						m.setCursor(i)
						break
					}
				}
			}
			m.clampCursor()
			m.setStatusMessage("All groups expanded", false)
		}
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
	}
	return m, nil
}

func (m Model) handleKeyOpenMarks() Model {
	m.overlay = overlayBookmarks
	m.overlayCursor = 0
	m.bookmarkFilter.Clear()
	// Every open starts with "don't load namespace"; a prior
	// session's Tab toggle must not leak in.
	m.bookmarkLoadNamespace = false
	return m
}

func (m Model) handleKeyHelp() Model {
	m.helpPreviousMode = modeExplorer
	m.mode = modeHelp
	m.helpScroll = 0
	m.helpFilter.Clear()
	m.helpSearchActive = false
	// Set contextual help mode based on the current overlay/view.
	switch m.overlay {
	case overlayBookmarks:
		m.helpContextMode = "Bookmarks"
	default:
		m.helpContextMode = "Navigation"
	}
	return m
}

func (m Model) handleKeyFilter() Model {
	m.filterActive = true
	m.filterBroadMode = false // each fresh filter session starts in name-only
	m.filterInput.Clear()
	m.filterText = ""
	m.setCursor(0)
	m.clampCursor()
	m.queryHistory.reset()
	return m
}

func (m Model) handleKeySearch() Model {
	m.searchActive = true
	m.searchBroadMode = false // each fresh search session starts in name-only
	m.searchInput.Clear()
	m.searchPrevCursor = m.cursor()
	m.queryHistory.reset()
	return m
}

func (m Model) handleKeyCommandBar() (Model, tea.Cmd) {
	m.commandBarActive = true
	m.commandBarInput.Clear()
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	m.commandHistory.reset()
	// Refresh the namespace cache if stale so namespaces created since
	// the last fetch (inside or outside the TUI) surface in completions.
	// The existing entry stays readable via namespaceNames() while the
	// refresh is in flight, keeping completions non-blank across the
	// TTL boundary.
	return m, m.ensureNamespaceCacheFresh()
}

func (m Model) handleKeyColumnToggle() Model {
	if m.nav.Level >= model.LevelResources {
		m.openColumnToggle()
	}
	return m
}

func (m Model) handleKeySecretToggle() (tea.Model, tea.Cmd) {
	m.showSecretValues = !m.showSecretValues
	if m.showSecretValues {
		m.setStatusMessage("Secret values VISIBLE", false)
	} else {
		m.setStatusMessage("Secret values HIDDEN", false)
	}
	return m, scheduleStatusClear()
}
