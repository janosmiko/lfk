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

// handleExplorerKey handles key events in the main explorer mode.
func (m Model) handleExplorerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := ui.ActiveKeybindings

	// Konami Code detection: track progress through the sequence.
	wasKonami := m.konamiActive
	m = m.checkKonami(msg)
	if m.konamiActive && !wasKonami {
		return m, scheduleKonamiClear()
	}

	// Clear pending 'g' state if any other key is pressed (vim-style gg).
	if m.pendingG && msg.String() != kb.JumpTop {
		m.pendingG = false
	}

	// Vim-style named marks: m<key> saves bookmark to slot, '<key> jumps to slot.
	if m.pendingMark {
		m.pendingMark = false
		key := msg.String()
		if len(key) == 1 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= '0' && key[0] <= '9')) {
			return m.bookmarkToSlot(key)
		}
		return m, nil
	}

	// Bookmark overwrite confirmation: Enter/y accepts, anything else cancels.
	if m.pendingBookmark != nil {
		bm := m.pendingBookmark
		m.pendingBookmark = nil
		switch msg.String() {
		case "enter", "y", "Y":
			return m.saveBookmark(*bm)
		}
		m.setStatusMessage("Cancelled", false)
		return m, scheduleStatusClear()
	}

	if mdl, cmd, handled := m.handleExplorerNavKey(msg); handled {
		return mdl, cmd
	}
	if mdl, cmd, handled := m.handleExplorerUIKey(msg); handled {
		return mdl, cmd
	}

	// Delegate remaining keys (actions, scrolling, tabs, editors, etc.)
	if ret, cmd, handled := m.handleExplorerActionKey(msg); handled {
		return ret, cmd
	}

	return m, nil
}

// handleExplorerNavKey handles navigation keys in explorer mode (movement, selection, enter).
func (m Model) handleExplorerNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings

	// Cancel active bulk operations on Ctrl+C or Esc before the normal
	// close-tab / back-navigation handling. This lets the user abort a
	// long-running delete/scale/restart without losing their tab.
	if m.bgtasks != nil && m.bgtasks.HasActiveMutations() {
		if key := msg.String(); key == "ctrl+c" || key == "esc" {
			m.bgtasks.CancelMutations()
			m.setStatusMessage("Cancelling...", false)
			return m, nil, true
		}
	}

	switch msg.String() {
	case "q":
		m.overlay = overlayQuitConfirm
		return m, nil, true
	case "ctrl+c":
		mdl, cmd := m.closeTabOrQuit()
		return mdl, cmd, true
	case "esc":
		mdl, cmd := m.handleExplorerEsc()
		return mdl, cmd, true
	case kb.Down, "down":
		if m.fullscreenDashboard {
			m.previewScroll++
			m.clampPreviewScroll()
			return m, nil, true
		}
		mdl, cmd := m.moveCursor(1)
		return mdl, cmd, true
	case kb.Up, "up":
		if m.fullscreenDashboard {
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil, true
		}
		mdl, cmd := m.moveCursor(-1)
		return mdl, cmd, true
	case kb.JumpTop:
		mdl, cmd := m.handleExplorerJumpTop()
		return mdl, cmd, true
	case kb.JumpBottom, "end":
		mdl, cmd := m.handleExplorerJumpBottom()
		return mdl, cmd, true
	case "home":
		mdl, cmd := m.handleExplorerHome()
		return mdl, cmd, true
	case kb.SelectRange:
		mdl := m.handleKeySelectRange()
		return mdl, nil, true
	case kb.ToggleSelect:
		mdl := m.handleKeyToggleSelect()
		return mdl, nil, true
	case kb.SelectAll:
		mdl := m.handleKeySelectAll()
		return mdl, nil, true
	case kb.Left, "left":
		if m.fullscreenDashboard {
			m.fullscreenDashboard = false
			m.previewScroll = 0
			m.setStatusMessage("Dashboard fullscreen OFF", false)
			return m, scheduleStatusClear(), true
		}
		mdl, cmd := m.navigateParent()
		return mdl, cmd, true
	case kb.Right, "right":
		mdl, cmd := m.navigateChild()
		return mdl, cmd, true
	case kb.Enter:
		mdl, cmd := m.enterFullView()
		return mdl, cmd, true
	case kb.NextMatch:
		mdl, cmd := m.handleKeyNextMatch()
		return mdl, cmd, true
	case kb.PrevMatch:
		mdl, cmd := m.handleKeyPrevMatch()
		return mdl, cmd, true
	}
	return m, nil, false
}

// handleExplorerEsc handles the Escape key in explorer mode. Cascades
// from transient content state (selection, search, filter) down to
// viewport state (fullscreen) and finally to navigation. Content state
// is what the user just did — peeling it off first matches the
// intent of "undo the most recent thing"; fullscreen is a more
// persistent toggle that the user is less likely to want exited
// before their search/filter work.
func (m Model) handleExplorerEsc() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		m.clearSelection()
		return m, nil
	}
	if m.searchInput.Value != "" {
		// Persisted search query (set by / + Enter) — clear it so the
		// match highlights and n/N navigation arm both disappear, but
		// stay on the current level.
		m.searchInput.Clear()
		return m, nil
	}
	if m.filterText != "" {
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		return m, m.loadPreview()
	}
	if m.fullscreenDashboard {
		m.fullscreenDashboard = false
		m.previewScroll = 0
		m.setStatusMessage("Dashboard fullscreen OFF", false)
		return m, scheduleStatusClear()
	}
	if m.nav.Level == model.LevelClusters {
		if len(m.tabs) > 1 {
			m.cancelActiveTabLogStreams()
			m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
			if m.activeTab > 0 {
				m.activeTab--
			}
			cmd := m.loadTab(m.activeTab)
			m.saveCurrentSession()
			if cmd != nil {
				return m, cmd
			}
			return m, m.loadPreview()
		}
		m.cancelAllTabLogStreams()
		return m, tea.Quit
	}
	return m.navigateParent()
}

// handleExplorerJumpTop handles g/gg (jump to top) in explorer mode.
func (m Model) handleExplorerJumpTop() (tea.Model, tea.Cmd) {
	if m.fullscreenDashboard {
		if m.pendingG {
			m.pendingG = false
			m.previewScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	}
	if m.pendingG {
		m.pendingG = false
		m.setCursor(0)
		m.clampCursor()
		m.syncExpandedGroup()
		return m, m.loadPreview()
	}
	m.pendingG = true
	return m, nil
}

// handleExplorerJumpBottom handles G (jump to bottom) in explorer mode.
func (m Model) handleExplorerJumpBottom() (tea.Model, tea.Cmd) {
	if m.fullscreenDashboard {
		m.previewScroll = 99999
		m.clampPreviewScroll()
		return m, nil
	}
	visible := m.visibleMiddleItems()
	if len(visible) > 0 {
		m.setCursor(len(visible) - 1)
	}
	m.syncExpandedGroup()
	return m, m.loadPreview()
}

// handleExplorerHome handles the Home key (jump to top) in explorer mode.
// Unlike JumpTop (vim "gg") this is a single-press action and does not
// participate in the pendingG two-key sequence.
func (m Model) handleExplorerHome() (tea.Model, tea.Cmd) {
	m.pendingG = false
	if m.fullscreenDashboard {
		m.previewScroll = 0
		return m, nil
	}
	m.setCursor(0)
	m.clampCursor()
	m.syncExpandedGroup()
	return m, m.loadPreview()
}

// handleExplorerUIKey handles UI toggle keys in explorer mode (filter, search, overlays, etc.).
func (m Model) handleExplorerUIKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.ThemeSelector:
		mdl := m.handleKeyThemeSelector()
		return mdl, nil, true
	case kb.NamespaceSelector:
		mdl, cmd := m.handleKeyNamespaceSelector()
		return mdl, cmd, true
	case kb.ActionMenu:
		mdl := m.openActionMenu()
		return mdl, nil, true
	case kb.WatchMode:
		mdl, cmd := m.handleKeyWatchMode()
		return mdl, cmd, true
	case kb.ExpandCollapse:
		mdl, cmd := m.handleKeyExpandCollapse()
		return mdl, cmd, true
	case kb.PinGroup:
		mdl, cmd := m.handleKeyPinGroup()
		return mdl, cmd, true
	case kb.OpenMarks:
		mdl := m.handleKeyOpenMarks()
		return mdl, nil, true
	case kb.SetMark:
		m.pendingMark = true
		return m, nil, true
	case kb.Help, "f1":
		mdl := m.handleKeyHelp()
		return mdl, nil, true
	case kb.Filter:
		mdl := m.handleKeyFilter()
		return mdl, nil, true
	case kb.Search:
		mdl := m.handleKeySearch()
		return mdl, nil, true
	case kb.CommandBar:
		mdl, cmd := m.handleKeyCommandBar()
		return mdl, cmd, true
	case kb.FinalizerSearch:
		m.openFinalizerSearch()
		return m, nil, true
	case kb.ColumnToggle:
		mdl := m.handleKeyColumnToggle()
		return mdl, nil, true
	case kb.ResourceMap:
		mdl, cmd := m.handleExplorerResourceMap()
		return mdl, cmd, true
	case kb.TogglePreview:
		mdl, cmd := m.handleExplorerTogglePreview()
		return mdl, cmd, true
	case kb.Fullscreen:
		mdl, cmd := m.handleExplorerFullscreen()
		return mdl, cmd, true
	case kb.SecretToggle:
		mdl, cmd := m.handleKeySecretToggle()
		return mdl, cmd, true
	}
	return m, nil, false
}

// handleExplorerResourceMap handles the resource map toggle key.
func (m Model) handleExplorerResourceMap() (tea.Model, tea.Cmd) {
	if m.nav.Level >= model.LevelResources {
		m.mapView = !m.mapView
		if m.mapView {
			m.fullYAMLPreview = false
			m.previewScroll = 0
			m.setStatusMessage("Resource map", false)
			return m, tea.Batch(m.loadResourceTree(), scheduleStatusClear())
		}
		m.resourceTree = nil
		m.setStatusMessage("Details preview", false)
		return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
	}
	return m, nil
}

// handleExplorerTogglePreview toggles between split view and full YAML preview.
func (m Model) handleExplorerTogglePreview() (tea.Model, tea.Cmd) {
	if sel := m.selectedMiddleItem(); sel != nil && m.nav.Level == model.LevelResourceTypes &&
		(sel.Extra == "__overview__" || sel.Extra == "__monitoring__") {
		return m, nil
	}
	m.fullYAMLPreview = !m.fullYAMLPreview
	m.mapView = false
	m.resourceTree = nil
	if m.fullYAMLPreview {
		m.setStatusMessage("YAML preview", false)
	} else {
		m.previewYAML = ""
		m.setStatusMessage("Details preview", false)
	}
	return m, tea.Batch(m.loadPreview(), scheduleStatusClear())
}

// handleExplorerFullscreen toggles fullscreen mode for the middle column or dashboard.
func (m Model) handleExplorerFullscreen() (tea.Model, tea.Cmd) {
	sel := m.selectedMiddleItem()
	if sel != nil && (sel.Extra == "__overview__" || sel.Extra == "__monitoring__") && m.nav.Level == model.LevelResourceTypes {
		m.fullscreenDashboard = !m.fullscreenDashboard
		m.previewScroll = 0
		if m.fullscreenDashboard {
			m.setStatusMessage("Dashboard fullscreen ON", false)
		} else {
			m.setStatusMessage("Dashboard fullscreen OFF", false)
		}
		return m, scheduleStatusClear()
	}
	m.fullscreenMiddle = !m.fullscreenMiddle
	if m.fullscreenMiddle {
		m.setStatusMessage("Fullscreen ON", false)
	} else {
		m.setStatusMessage("Fullscreen OFF", false)
	}
	return m, scheduleStatusClear()
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
	m.overlayCursor = 0
	m.overlayItems = nil // will be populated when namespacesLoadedMsg arrives
	ui.ResetOverlayNsScroll()
	m.loading = true
	m.nsSelectionModified = false
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

func (m Model) handleKeyPinGroup() (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelResourceTypes {
		sel := m.selectedMiddleItem()
		if sel == nil || sel.Category == "" {
			return m, nil
		}
		// Don't allow pinning core categories.
		if model.IsCoreCategory(sel.Category) {
			m.setStatusMessage("Cannot pin built-in category", true)
			return m, scheduleStatusClear()
		}
		pinned := togglePinnedGroup(m.pinnedState, m.nav.Context, sel.Category)
		_ = savePinnedState(m.pinnedState)
		m.applyPinnedGroups()
		// Refresh the resource type list to reflect new ordering.
		if pinned {
			m.setStatusMessage(fmt.Sprintf("Pinned: %s", sel.Category), false)
		} else {
			m.setStatusMessage(fmt.Sprintf("Unpinned: %s", sel.Category), false)
		}
		// Reload resource types to apply the new pin order.
		return m, tea.Batch(m.loadResourceTypes(), scheduleStatusClear())
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
