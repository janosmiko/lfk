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

	// Handle error log overlay first (independent of regular overlays).
	if m.overlayErrorLog {
		return m.handleErrorLogOverlayKey(msg)
	}

	// Handle overlays first.
	if m.overlay != overlayNone {
		return m.handleOverlayKey(msg)
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
	kb := ui.ActiveKeybindings
	if m.mode != modeExplorer && m.mode != modeExec && !m.yamlSearchMode && !m.logSearchActive && !m.helpSearchActive && !m.explainSearchActive {
		switch msg.String() {
		case kb.NextTab:
			if len(m.tabs) > 1 {
				m.saveCurrentTab() // save mode + log state BEFORE switching
				next := (m.activeTab + 1) % len(m.tabs)
				if cmd := m.loadTab(next); cmd != nil {
					return m, cmd
				}
				if m.mode == modeExplorer {
					return m, m.loadPreview()
				}
				if m.mode == modeLogs && m.logCh != nil {
					return m, m.waitForLogLine()
				}
				if m.mode == modeExec && m.execPTY != nil {
					return m, m.scheduleExecTick()
				}
				return m, nil
			}
		case kb.PrevTab:
			if len(m.tabs) > 1 {
				m.saveCurrentTab()
				prev := (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
				if cmd := m.loadTab(prev); cmd != nil {
					return m, cmd
				}
				if m.mode == modeExplorer {
					return m, m.loadPreview()
				}
				if m.mode == modeLogs && m.logCh != nil {
					return m, m.waitForLogLine()
				}
				if m.mode == modeExec && m.execPTY != nil {
					return m, m.scheduleExecTick()
				}
				return m, nil
			}
		case kb.NewTab:
			if m.mode != modeHelp { // 't' is not used as a regular key in fullscreen modes
				if len(m.tabs) >= 9 {
					m.setStatusMessage("Max 9 tabs", true)
					return m, scheduleStatusClear()
				}
				m.saveCurrentTab()
				newTab := m.cloneCurrentTab()
				// New tab always starts in explorer mode.
				newTab.mode = modeExplorer
				newTab.logLines = nil
				newTab.logCancel = nil
				newTab.logCh = nil
				insertAt := m.activeTab + 1
				m.tabs = append(m.tabs[:insertAt], append([]TabState{newTab}, m.tabs[insertAt:]...)...)
				m.activeTab = insertAt
				m.loadTab(m.activeTab) // cloned tab is always fully loaded, ignore returned cmd
				m.setStatusMessage(fmt.Sprintf("Tab %d created", m.activeTab+1), false)
				return m, scheduleStatusClear()
			}
		}
	}

	// Embedded terminal mode: forward keys to PTY.
	if m.mode == modeExec {
		return m.handleExecKey(msg)
	}

	// In help view mode.
	if m.mode == modeHelp {
		return m.handleHelpKey(msg)
	}

	// In log viewer mode.
	if m.mode == modeLogs {
		return m.handleLogKey(msg)
	}

	// In diff view mode.
	if m.mode == modeDiff {
		return m.handleDiffKey(msg)
	}

	// In describe view mode.
	if m.mode == modeDescribe {
		return m.handleDescribeKey(msg)
	}

	// In explain view mode: handle search input before general keys.
	if m.mode == modeExplain && m.explainSearchActive {
		return m.handleExplainSearchKey(msg)
	}
	if m.mode == modeExplain {
		return m.handleExplainKey(msg)
	}

	// In YAML view mode.
	if m.mode == modeYAML {
		return m.handleYAMLKey(msg)
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
		// Invalid slot key — ignore silently.
		return m, nil
	}

	// Bookmark overwrite confirmation: y accepts, anything else cancels.
	if m.pendingBookmark != nil {
		bm := m.pendingBookmark
		m.pendingBookmark = nil
		if msg.String() == "y" || msg.String() == "Y" {
			return m.saveBookmark(*bm)
		}
		m.setStatusMessage("Cancelled", false)
		return m, scheduleStatusClear()
	}

	switch msg.String() {
	case "q":
		m.overlay = overlayQuitConfirm
		return m, nil

	case "ctrl+c":
		return m.closeTabOrQuit()

	case kb.ThemeSelector:
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
		return m, nil

	case "esc":
		if m.fullscreenDashboard {
			m.fullscreenDashboard = false
			m.previewScroll = 0
			m.setStatusMessage("Dashboard fullscreen OFF", false)
			return m, scheduleStatusClear()
		}
		if m.hasSelection() {
			m.clearSelection()
			return m, nil
		}
		if m.filterText != "" {
			m.filterText = ""
			m.setCursor(0)
			m.clampCursor()
			return m, m.loadPreview()
		}
		if m.nav.Level == model.LevelClusters {
			if len(m.tabs) > 1 {
				// Close current tab instead of quitting when multiple tabs are open.
				m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
				if m.activeTab > 0 {
					m.activeTab--
				}
				// Load the surviving tab BEFORE saving session, so saveCurrentTab
				// writes the surviving tab's data (not the closed tab's stale state).
				cmd := m.loadTab(m.activeTab)
				m.saveCurrentSession()
				if cmd != nil {
					return m, cmd
				}
				return m, m.loadPreview()
			}
			return m, tea.Quit
		}
		return m.navigateParent()

	case kb.Down, "down":
		if m.fullscreenDashboard {
			m.previewScroll++
			m.clampPreviewScroll()
			return m, nil
		}
		return m.moveCursor(1)

	case kb.Up, "up":
		if m.fullscreenDashboard {
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		}
		return m.moveCursor(-1)

	case kb.JumpTop:
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

	case kb.JumpBottom:
		if m.fullscreenDashboard {
			m.previewScroll = 99999 // will be clamped
			m.clampPreviewScroll()
			return m, nil
		}
		visible := m.visibleMiddleItems()
		if len(visible) > 0 {
			m.setCursor(len(visible) - 1)
		}
		m.syncExpandedGroup()
		return m, m.loadPreview()

	case kb.SelectRange: // ctrl+space: region selection
		if m.nav.Level < model.LevelResources {
			return m, nil
		}
		items := m.visibleMiddleItems()
		if len(items) == 0 {
			return m, nil
		}
		cur := m.cursor()
		if m.selectionAnchor < 0 {
			// No anchor set — toggle current item and set anchor.
			if sel := m.selectedMiddleItem(); sel != nil {
				m.toggleSelection(*sel)
				m.selectionAnchor = cur
			}
			return m, nil
		}
		// Select range from anchor to cursor (inclusive).
		lo, hi := m.selectionAnchor, cur
		if lo > hi {
			lo, hi = hi, lo
		}
		for i := lo; i <= hi && i < len(items); i++ {
			m.selectedItems[selectionKey(items[i])] = true
		}
		return m, nil

	case kb.ToggleSelect:
		// Toggle selection on current item and move cursor down.
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
			return m, nil
		}
		return m, nil

	case kb.SelectAll:
		// Select/deselect all visible items.
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
			}
			m.selectionAnchor = -1
			return m, nil
		}
		return m, nil

	case kb.Left, "left":
		if m.fullscreenDashboard {
			m.fullscreenDashboard = false
			m.previewScroll = 0
			m.setStatusMessage("Dashboard fullscreen OFF", false)
			return m, scheduleStatusClear()
		}
		return m.navigateParent()

	case kb.Right, "right":
		return m.navigateChild()

	case kb.Enter:
		return m.enterFullView()

	case kb.NextMatch:
		// Jump to next search match.
		if m.searchInput.Value != "" {
			m.jumpToSearchMatch(m.cursor() + 1)
			m.syncExpandedGroup()
			return m, m.loadPreview()
		}
		return m, nil

	case kb.PrevMatch:
		// Jump to previous search match.
		if m.searchInput.Value != "" {
			m.jumpToPrevSearchMatch(m.cursor() - 1)
			m.syncExpandedGroup()
			return m, m.loadPreview()
		}
		return m, nil

	case kb.NamespaceSelector:
		// Open namespace selector immediately with loading state.
		m.overlay = overlayNamespace
		m.overlayFilter.Clear()
		m.overlayCursor = 0
		m.overlayItems = nil // will be populated when namespacesLoadedMsg arrives
		ui.ResetOverlayNsScroll()
		m.loading = true
		m.nsSelectionModified = false
		return m, m.loadNamespaces()

	case kb.ActionMenu:
		return m.openActionMenu()

	case kb.WatchMode:
		m.watchMode = !m.watchMode
		if m.watchMode {
			m.setStatusMessage("Watch mode ON (refresh every 2s)", false)
			return m, tea.Batch(scheduleWatchTick(m.watchInterval), scheduleStatusClear())
		}
		m.setStatusMessage("Watch mode OFF", false)
		return m, scheduleStatusClear()

	case kb.ExpandCollapse:
		// Toggle expand/collapse all groups at resource types level.
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

	case kb.PinGroup:
		// Pin/unpin CRD group at resource types level.
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

	case kb.OpenMarks:
		// Open bookmarks overlay immediately; slot keys (a-z, 0-9) jump from within the overlay.
		m.overlay = overlayBookmarks
		m.overlayCursor = 0
		m.bookmarkFilter.Clear()
		return m, nil

	case kb.SetMark:
		// Vim-style set mark: wait for slot key.
		m.pendingMark = true
		return m, nil

	case kb.Help:
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
		return m, nil

	case kb.Filter:
		m.filterActive = true
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		return m, nil

	case kb.Search:
		m.searchActive = true
		m.searchInput.Clear()
		m.searchPrevCursor = m.cursor()
		return m, nil

	case kb.CommandBar:
		// Open kubectl command bar.
		m.commandBarActive = true
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		m.commandHistory.reset()
		return m, nil

	case kb.FinalizerSearch:
		// Open finalizer search overlay.
		m.openFinalizerSearch()
		return m, nil

	case kb.ColumnToggle:
		// Open column toggle overlay.
		if m.nav.Level >= model.LevelResources {
			m.openColumnToggle()
		}
		return m, nil

	case kb.ResourceMap:
		// Toggle resource relationship map in the right column.
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

	case kb.TogglePreview:
		// No preview toggle on dashboard views.
		if sel := m.selectedMiddleItem(); sel != nil && m.nav.Level == model.LevelResourceTypes &&
			(sel.Extra == "__overview__" || sel.Extra == "__monitoring__") {
			return m, nil
		}
		// Toggle between split view (children + details) and full YAML preview.
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

	case kb.Fullscreen:
		// If standing on Cluster Dashboard or Monitoring, toggle fullscreen dashboard instead.
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
		// Toggle fullscreen middle column.
		m.fullscreenMiddle = !m.fullscreenMiddle
		if m.fullscreenMiddle {
			m.setStatusMessage("Fullscreen ON", false)
		} else {
			m.setStatusMessage("Fullscreen OFF", false)
		}
		return m, scheduleStatusClear()

	case kb.SecretToggle:
		// Toggle secret value visibility.
		m.showSecretValues = !m.showSecretValues
		if m.showSecretValues {
			m.setStatusMessage("Secret values VISIBLE", false)
		} else {
			m.setStatusMessage("Secret values HIDDEN", false)
		}
		return m, scheduleStatusClear()

	default:
		// Delegate remaining keys (actions, scrolling, tabs, editors, etc.)
		// to the action key handler in update_keys_actions.go.
		if ret, cmd, handled := m.handleExplorerActionKey(msg); handled {
			return ret, cmd
		}
	}

	return m, nil
}
