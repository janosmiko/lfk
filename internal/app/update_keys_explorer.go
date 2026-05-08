package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleExplorerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	kb := ui.ActiveKeybindings

	wasKonami := m.konamiActive
	m = m.checkKonami(msg)
	if m.konamiActive && !wasKonami {
		return m, scheduleKonamiClear()
	}

	if m.pendingG && msg.String() != kb.JumpTop {
		m.pendingG = false
	}

	if m.pendingMark {
		m.pendingMark = false
		key := msg.String()
		if len(key) == 1 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= '0' && key[0] <= '9')) {
			return m.bookmarkToSlot(key)
		}
		return m, nil
	}

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

	if ret, cmd, handled := m.handleExplorerActionKey(msg); handled {
		return ret, cmd
	}

	return m, nil
}

func (m Model) handleExplorerNavKey(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	kb := ui.ActiveKeybindings

	if m.scheduler != nil && m.scheduler.HasActiveMutations() {
		if key := msg.String(); key == "ctrl+c" || key == "esc" {
			m.scheduler.CancelMutations()
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
	case kb.ReadOnlyToggle:
		mdl, cmd := m.handleKeyReadOnlyToggle()
		return mdl, cmd, true
	case kb.ClusterColorPicker:
		// Gate on Level=Clusters so the bare-letter default "c" doesn't
		// silently shadow other handlers at deeper levels — the handler
		// also self-gates as a defensive belt-and-braces measure.
		if m.nav.Level != model.LevelClusters {
			break
		}
		mdl, cmd := m.handleKeyClusterColorPicker()
		return mdl, cmd, true
	case kb.LocalClusterManager:
		// Gate on Level=Clusters: the manager only makes sense when the
		// cluster picker is visible, and Ctrl+N would otherwise shadow
		// nothing at deeper levels (it has no current default binding
		// elsewhere) — but the gate keeps room for future bindings.
		if m.nav.Level != model.LevelClusters {
			break
		}
		mdl, cmd := m.openLocalClusterManager()
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

func (m Model) handleExplorerEsc() (tea.Model, tea.Cmd) {
	if m.hasSelection() {
		m.clearSelection()
		return m, nil
	}
	if m.searchInput.Value != "" {
		m.searchInput.Clear()
		return m, nil
	}
	if m.filterText != "" {
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		return m, m.loadPreview()
	}
	if m.activeFilterPreset != nil {
		// Mirror handleExplorerActionKeyFilterPresets so Esc and a second
		// press of `.` are interchangeable: drop the preset, restore the
		// pre-filter list, and tell the user which preset was cleared.
		name := m.activeFilterPreset.Name
		m.activeFilterPreset = nil
		m.setMiddleItems(m.unfilteredMiddleItems)
		m.unfilteredMiddleItems = nil
		m.setCursor(0)
		m.clampCursor()
		m.setStatusMessage("Filter cleared: "+name, false)
		return m, tea.Batch(scheduleStatusClear(), m.loadPreview())
	}
	if m.fullscreenDashboard {
		m.fullscreenDashboard = false
		m.previewScroll = 0
		m.setStatusMessage("Dashboard fullscreen OFF", false)
		return m, scheduleStatusClear()
	}
	if m.nav.Level == model.LevelClusters && len(m.tabs) > 1 {
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
	return m, nil
}

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
	case kb.OrphanOverlay:
		mdl, cmd := m.openOrphansOverlay()
		return mdl, cmd, true
	}
	return m, nil, false
}

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

func (m Model) handleKeyPinGroup() (tea.Model, tea.Cmd) {
	if m.nav.Level == model.LevelResourceTypes {
		sel := m.selectedMiddleItem()
		if sel == nil || sel.Category == "" {
			return m, nil
		}
		if model.IsCoreCategory(sel.Category) {
			m.setStatusMessage("Cannot pin built-in category", true)
			return m, scheduleStatusClear()
		}
		pinned := togglePinnedGroup(m.pinnedState, m.nav.Context, sel.Category)
		if err := savePinnedState(m.pinnedState); err != nil {
			// Roll back the in-memory toggle so runtime state matches what
			// is actually persisted to disk; togglePinnedGroup is its own
			// inverse, so calling it again undoes the mutation.
			_ = togglePinnedGroup(m.pinnedState, m.nav.Context, sel.Category)
			m.setStatusMessage(fmt.Sprintf("Failed to save pinned groups: %v", err), true)
			return m, scheduleStatusClear()
		}
		m.applyPinnedGroups()
		if pinned {
			m.setStatusMessage(fmt.Sprintf("Pinned: %s", sel.Category), false)
		} else {
			m.setStatusMessage(fmt.Sprintf("Unpinned: %s", sel.Category), false)
		}
		return m, tea.Batch(m.loadResourceTypes(), scheduleStatusClear())
	}
	return m, nil
}
