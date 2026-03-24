package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
)

// --- Bookmark handlers ---

// bookmarkToSlot saves the current location as a named mark in the given slot (a-z, 0-9).
// If a bookmark already exists in that slot, it prompts for confirmation.
func (m Model) bookmarkToSlot(slot string) (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Navigate to a resource type first", true)
		return m, scheduleStatusClear()
	}

	isGlobal := len(slot) == 1 && slot[0] >= 'A' && slot[0] <= 'Z'

	// Global bookmarks include the context in the name and save it for
	// cross-cluster navigation. Local bookmarks are context-independent.
	var parts []string
	if isGlobal {
		parts = append(parts, m.nav.Context)
	}
	if m.nav.ResourceType.DisplayName != "" {
		parts = append(parts, m.nav.ResourceType.DisplayName)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	name := strings.Join(parts, " > ")

	var ns string
	var nsList []string
	if m.allNamespaces {
		ns = ""
	} else if len(m.selectedNamespaces) > 1 {
		// Multiple namespaces selected — store them all.
		nsList = make([]string, 0, len(m.selectedNamespaces))
		for n := range m.selectedNamespaces {
			nsList = append(nsList, n)
		}
		sort.Strings(nsList)
		ns = nsList[0] // primary namespace for backward compat display
	} else {
		ns = m.namespace
	}

	bmContext := ""
	if isGlobal {
		bmContext = m.nav.Context
	}

	bm := model.Bookmark{
		Name:         name,
		Context:      bmContext,
		Namespace:    ns,
		Namespaces:   nsList,
		ResourceType: m.nav.ResourceType.ResourceRef(),
		ResourceName: m.nav.ResourceName,
		Slot:         slot,
		Global:       isGlobal,
	}

	// Check if slot is already in use; if so, ask for confirmation.
	for _, b := range m.bookmarks {
		if b.Slot == slot {
			m.pendingBookmark = &bm
			m.setStatusMessage(fmt.Sprintf("Mark '%s' exists (%s). Overwrite? (y/n)", slot, b.Name), true)
			return m, nil
		}
	}

	return m.saveBookmark(bm)
}

// saveBookmark persists a bookmark, replacing any existing one in the same slot.
func (m Model) saveBookmark(bm model.Bookmark) (tea.Model, tea.Cmd) {
	for i, b := range m.bookmarks {
		if b.Slot == bm.Slot {
			m.bookmarks = append(m.bookmarks[:i], m.bookmarks[i+1:]...)
			break
		}
	}
	m.bookmarks = append(m.bookmarks, bm)

	if err := saveBookmarks(m.bookmarks); err != nil {
		m.setStatusMessage("Failed to save mark: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage(fmt.Sprintf("Mark '%s' set: %s", bm.Slot, bm.Name), false)
	return m, scheduleStatusClear()
}

// jumpToSlot navigates to the bookmark saved in the given slot.
func (m Model) jumpToSlot(slot string) (tea.Model, tea.Cmd) {
	for _, bm := range m.bookmarks {
		if bm.Slot == slot {
			return m.navigateToBookmark(bm)
		}
	}
	m.setStatusMessage(fmt.Sprintf("Mark '%s' not set", slot), true)
	return m, scheduleStatusClear()
}

// filteredBookmarks returns bookmarks matching the current bookmark filter.
func (m *Model) filteredBookmarks() []model.Bookmark {
	if m.bookmarkFilter.Value == "" {
		return m.bookmarks
	}
	filter := strings.ToLower(m.bookmarkFilter.Value)
	var filtered []model.Bookmark
	for _, bm := range m.bookmarks {
		if strings.Contains(strings.ToLower(bm.Name), filter) {
			filtered = append(filtered, bm)
		}
	}
	return filtered
}

// bookmarkDeleteCurrent removes the bookmark at the current cursor position.
func (m *Model) bookmarkDeleteCurrent() tea.Cmd {
	filtered := m.filteredBookmarks()
	if len(filtered) == 0 || m.overlayCursor < 0 || m.overlayCursor >= len(filtered) {
		return nil
	}
	target := filtered[m.overlayCursor]
	for i, bm := range m.bookmarks {
		if bm.Slot == target.Slot {
			m.bookmarks = removeBookmark(m.bookmarks, i)
			break
		}
	}
	_ = saveBookmarks(m.bookmarks)
	newFiltered := m.filteredBookmarks()
	if m.overlayCursor >= len(newFiltered) && m.overlayCursor > 0 {
		m.overlayCursor--
	}
	m.setStatusMessage("Removed bookmark: "+target.Name, false)
	if len(m.bookmarks) == 0 {
		m.overlay = overlayNone
	}
	return scheduleStatusClear()
}

// bookmarkDeleteAll removes all bookmarks (or all filtered bookmarks if a filter is active).
func (m *Model) bookmarkDeleteAll() tea.Cmd {
	filtered := m.filteredBookmarks()
	if len(filtered) == 0 {
		return nil
	}
	if m.bookmarkFilter.Value == "" {
		// Delete all bookmarks.
		m.bookmarks = nil
	} else {
		// Delete only the filtered bookmarks.
		filterSet := make(map[string]bool)
		for _, bm := range filtered {
			filterSet[bm.Slot] = true
		}
		var remaining []model.Bookmark
		for _, bm := range m.bookmarks {
			key := bm.Slot
			if !filterSet[key] {
				remaining = append(remaining, bm)
			}
		}
		m.bookmarks = remaining
	}
	_ = saveBookmarks(m.bookmarks)
	m.overlayCursor = 0
	count := len(filtered)
	m.setStatusMessage(fmt.Sprintf("Removed %d bookmark(s)", count), false)
	if len(m.bookmarks) == 0 {
		m.overlay = overlayNone
	}
	return scheduleStatusClear()
}

// handleBookmarkOverlayKey handles key events in the bookmark overlay.
func (m Model) handleBookmarkOverlayKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	filtered := m.filteredBookmarks()

	switch m.bookmarkSearchMode {
	case bookmarkModeFilter:
		return m.handleBookmarkFilterMode(msg)
	case bookmarkModeConfirmDelete:
		return m.handleBookmarkConfirmDelete(msg)
	case bookmarkModeConfirmDeleteAll:
		return m.handleBookmarkConfirmDeleteAll(msg)
	default:
		return m.handleBookmarkNormalMode(msg, filtered)
	}
}

// handleBookmarkNormalMode handles keys when the bookmark overlay is in normal navigation mode.
func (m Model) handleBookmarkNormalMode(msg tea.KeyMsg, filtered []model.Bookmark) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.overlay = overlayNone
		m.bookmarkFilter.Clear()
		m.bookmarkSearchMode = bookmarkModeNormal
		return m, nil
	case "enter":
		if len(filtered) > 0 && m.overlayCursor >= 0 && m.overlayCursor < len(filtered) {
			return m.navigateToBookmark(filtered[m.overlayCursor])
		}
		return m, nil
	case "j", "down", "ctrl+n":
		if m.overlayCursor < len(filtered)-1 {
			m.overlayCursor++
		}
		return m, nil
	case "k", "up", "ctrl+p":
		if m.overlayCursor > 0 {
			m.overlayCursor--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.overlayCursor = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		if len(filtered) > 0 {
			m.overlayCursor = len(filtered) - 1
		}
		return m, nil
	case "ctrl+d":
		m.overlayCursor += 10
		if m.overlayCursor >= len(filtered) {
			m.overlayCursor = len(filtered) - 1
		}
		return m, nil
	case "ctrl+u":
		m.overlayCursor -= 10
		if m.overlayCursor < 0 {
			m.overlayCursor = 0
		}
		return m, nil
	case "ctrl+f":
		m.overlayCursor += 20
		if m.overlayCursor >= len(filtered) {
			m.overlayCursor = len(filtered) - 1
		}
		return m, nil
	case "ctrl+b":
		m.overlayCursor -= 20
		if m.overlayCursor < 0 {
			m.overlayCursor = 0
		}
		return m, nil
	case "/":
		m.bookmarkSearchMode = bookmarkModeFilter
		m.bookmarkFilter.Clear()
		return m, nil
	case "D":
		if len(filtered) > 0 && m.overlayCursor >= 0 && m.overlayCursor < len(filtered) {
			target := filtered[m.overlayCursor]
			label := target.Name
			if target.Slot != "" {
				label = fmt.Sprintf("'%s' (%s)", target.Slot, target.Name)
			}
			m.bookmarkSearchMode = bookmarkModeConfirmDelete
			m.setStatusMessage(fmt.Sprintf("Delete mark %s? (y/n)", label), true)
		}
		return m, nil
	case "ctrl+x":
		if len(filtered) > 0 {
			count := len(filtered)
			m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
			m.setStatusMessage(fmt.Sprintf("Delete %d bookmark(s)? (y/n)", count), true)
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		// Slot-key shortcut: pressing a-z, A-Z, or 0-9 jumps directly to that named mark.
		key := msg.String()
		if len(key) == 1 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= '0' && key[0] <= '9')) {
			return m.jumpToSlot(key)
		}
	}
	return m, nil
}

// handleBookmarkFilterMode handles keys when the bookmark overlay is in filter input mode.
func (m Model) handleBookmarkFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.bookmarkSearchMode = bookmarkModeNormal
		m.bookmarkFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case "enter":
		m.bookmarkSearchMode = bookmarkModeNormal
		return m, nil
	case "backspace":
		if len(m.bookmarkFilter.Value) > 0 {
			m.bookmarkFilter.Backspace()
			m.overlayCursor = 0
		}
		return m, nil
	case "ctrl+w":
		m.bookmarkFilter.DeleteWord()
		m.overlayCursor = 0
		return m, nil
	case "ctrl+a":
		m.bookmarkFilter.Home()
		return m, nil
	case "ctrl+e":
		m.bookmarkFilter.End()
		return m, nil
	case "left":
		m.bookmarkFilter.Left()
		return m, nil
	case "right":
		m.bookmarkFilter.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.bookmarkFilter.Insert(key)
			m.overlayCursor = 0
		}
		return m, nil
	}
}

// handleBookmarkConfirmDelete handles y/n confirmation for single bookmark deletion.
func (m Model) handleBookmarkConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.bookmarkSearchMode = bookmarkModeNormal
	switch msg.String() {
	case "y", "Y":
		cmd := m.bookmarkDeleteCurrent()
		return m, cmd
	default:
		m.setStatusMessage("Cancelled", false)
		return m, scheduleStatusClear()
	}
}

// handleBookmarkConfirmDeleteAll handles y/n confirmation for deleting all bookmarks.
func (m Model) handleBookmarkConfirmDeleteAll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.bookmarkSearchMode = bookmarkModeNormal
	switch msg.String() {
	case "y", "Y":
		cmd := m.bookmarkDeleteAll()
		return m, cmd
	default:
		m.setStatusMessage("Cancelled", false)
		return m, scheduleStatusClear()
	}
}

// navigateToBookmark jumps to the location described by a bookmark.
func (m Model) navigateToBookmark(bm model.Bookmark) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.bookmarkFilter.Clear()

	// For local bookmarks, stay in the current cluster context.
	// For global bookmarks, switch to the bookmark's saved context.
	effectiveContext := bm.Context
	if !bm.Global {
		effectiveContext = m.nav.Context
	}

	rt, ok := model.FindResourceTypeIn(bm.ResourceType, m.discoveredCRDs[effectiveContext])
	if !ok {
		m.setStatusMessage("Resource type not found in current cluster", true)
		return m, scheduleStatusClear()
	}

	// Switch context (global bookmarks change cluster, local bookmarks keep current).
	m.nav.Context = effectiveContext
	m.dashboardPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedGroups()

	// Set namespace(s).
	if bm.Namespace == "" && len(bm.Namespaces) == 0 {
		m.allNamespaces = true
		m.selectedNamespaces = nil
	} else if len(bm.Namespaces) > 1 {
		// Multiple namespaces stored.
		m.allNamespaces = false
		m.namespace = bm.Namespaces[0]
		m.selectedNamespaces = make(map[string]bool, len(bm.Namespaces))
		for _, ns := range bm.Namespaces {
			m.selectedNamespaces[ns] = true
		}
	} else {
		// Single namespace (legacy or single-select).
		m.allNamespaces = false
		ns := bm.Namespace
		if len(bm.Namespaces) == 1 {
			ns = bm.Namespaces[0]
		}
		m.namespace = ns
		m.selectedNamespaces = map[string]bool{ns: true}
	}

	// Navigate to resource type level first, then optionally deeper.
	m.nav.ResourceType = rt
	m.nav.ResourceName = bm.ResourceName

	// Navigate to resources level (optionally with a specific resource selected).
	m.nav.Level = model.LevelResources

	// Reset navigation state that doesn't apply.
	m.nav.OwnedName = ""
	m.nav.Namespace = ""

	// Reset column data and history; we'll rebuild from the target level.
	m.leftItems = nil
	m.leftItemsHistory = nil
	m.middleItems = nil
	m.rightItems = nil
	m.clearRight()

	// Rebuild left items history: clusters -> resource types.
	// Load contexts as the base left column.
	contexts, _ := m.client.GetContexts()
	var resourceTypes []model.Item
	if crds := m.discoveredCRDs[effectiveContext]; len(crds) > 0 {
		resourceTypes = model.MergeWithCRDs(crds)
	} else {
		resourceTypes = model.FlattenedResourceTypes()
	}

	// Set up history: at LevelResources, leftItemsHistory has [contexts], leftItems = resourceTypes.
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes

	// Reset cursors, then set the parent (resource types) cursor to the
	// correct position so that pressing 'h' returns to the right item.
	m.cursors = [5]int{}
	m.cursorMemory = make(map[string]int)
	m.itemCache = make(map[string][]model.Item)
	m.setCursor(0)

	// Remember the resource type position at the parent level (navKey = context only).
	rtRef := rt.ResourceRef()
	for i, item := range resourceTypes {
		if item.Extra == rtRef {
			m.cursorMemory[effectiveContext] = i
			break
		}
	}

	m.cancelAndReset()
	m.requestGen++
	m.loading = true
	m.filterText = ""
	m.filterActive = false
	m.searchActive = false

	m.setStatusMessage("Jumped to: "+bm.Name, false)
	return m, tea.Batch(m.loadResources(false), scheduleStatusClear())
}

// restoreSession applies the pending session state after contexts have been loaded.
// It navigates to the saved context and optionally to the resource type level,
// similar to how bookmark navigation works.
//
// When the session contains multiple tabs (Tabs field), the non-active tabs are
// created with their navigation state pre-populated and needsLoad=true so that
// their data is fetched lazily when the user switches to them.
func (m Model) restoreSession(contexts []model.Item) (tea.Model, tea.Cmd) {
	sess := m.pendingSession
	m.pendingSession = nil
	m.sessionRestored = true

	// If the session has multi-tab data, delegate to the multi-tab restore path.
	if len(sess.Tabs) > 0 {
		return m.restoreMultiTabSession(sess, contexts)
	}

	// Legacy single-tab session restore (no Tabs field).
	return m.restoreSingleTabSession(sess, contexts)
}

// restoreSingleTabSession handles the legacy session format that stores a single
// navigation path. This is the original restore logic preserved for backward
// compatibility with session files that predate multi-tab support.
func (m Model) restoreSingleTabSession(sess *SessionState, contexts []model.Item) (tea.Model, tea.Cmd) {
	// Verify the saved context still exists in the loaded context list.
	if !contextInList(sess.Context, contexts) {
		return m, m.loadPreview()
	}

	// Navigate into the saved context (same as navigateChild at LevelClusters).
	m.nav.Context = sess.Context
	m.applyPinnedGroups()
	m.nav.Level = model.LevelResourceTypes

	// Set up left column history: contexts list becomes the breadcrumb.
	m.leftItemsHistory = nil
	m.leftItems = contexts

	// Load resource types for the middle column.
	m.middleItems = model.FlattenedResourceTypes()
	m.itemCache[m.navKey()] = m.middleItems
	m.clearRight()

	// Restore namespace settings from session.
	applySessionNamespaces(&m, sess.AllNamespaces, sess.Namespace, sess.SelectedNamespaces)

	cmds := []tea.Cmd{m.discoverCRDs(sess.Context)}

	// If a resource type was saved, navigate deeper.
	if sess.ResourceType != "" {
		rt, ok := model.FindResourceType(sess.ResourceType)
		if ok {
			// Save cursor position at the resource types level so navigating
			// back (h) restores the cursor to the correct resource type.
			rtRef := rt.ResourceRef()
			for i, item := range m.middleItems {
				if item.Extra == rtRef {
					// navKey at ResourceTypes level = context only.
					m.cursorMemory[m.nav.Context] = i
					break
				}
			}

			// Push resource types into history and navigate to resources level.
			m.leftItemsHistory = [][]model.Item{contexts}
			m.leftItems = m.middleItems
			m.nav.ResourceType = rt
			m.nav.Level = model.LevelResources
			m.middleItems = nil
			m.clearRight()
			m.setCursor(0)
			m.loading = true

			if sess.ResourceName != "" {
				m.pendingTarget = sess.ResourceName
			}

			cmds = append(cmds, m.loadResources(false))
			return m, tea.Batch(cmds...)
		}
	}

	// No resource type or not found: stay at resource types level.
	m.clampCursor()
	cmds = append(cmds, m.loadPreview())
	return m, tea.Batch(cmds...)
}

// restoreMultiTabSession creates tab entries from the session's Tabs slice,
// fully restores the active tab, and marks the rest as needsLoad so their
// data is fetched on first switch.
func (m Model) restoreMultiTabSession(sess *SessionState, contexts []model.Item) (tea.Model, tea.Cmd) {
	activeIdx := sess.ActiveTab
	if activeIdx < 0 || activeIdx >= len(sess.Tabs) {
		activeIdx = 0
	}

	// Validate the active tab's context exists; if not fall back to normal startup.
	activeSess := sess.Tabs[activeIdx]
	if !contextInList(activeSess.Context, contexts) {
		return m, m.loadPreview()
	}

	// Build TabState entries for every tab.
	tabs := make([]TabState, 0, len(sess.Tabs))
	for i, st := range sess.Tabs {
		tab := buildSessionTabState(&st)
		if i != activeIdx {
			// Non-active tabs are lazily loaded on first switch.
			tab.needsLoad = true
		}
		tabs = append(tabs, tab)
	}

	m.tabs = tabs
	m.activeTab = activeIdx

	// Fully restore the active tab into the Model (same logic as single-tab).
	return m.restoreSingleTabSession(&SessionState{
		Context:            activeSess.Context,
		Namespace:          activeSess.Namespace,
		AllNamespaces:      activeSess.AllNamespaces,
		SelectedNamespaces: activeSess.SelectedNamespaces,
		ResourceType:       activeSess.ResourceType,
		ResourceName:       activeSess.ResourceName,
	}, contexts)
}

// buildSessionTabState creates a TabState with navigation fields populated
// from a SessionTab. The tab has no loaded items yet; those are fetched
// when the tab becomes active (needsLoad).
func buildSessionTabState(st *SessionTab) TabState {
	tab := TabState{
		nav: model.NavigationState{
			Context: st.Context,
		},
		splitPreview:      true,
		watchMode:         true,
		warningEventsOnly: true,
		allGroupsExpanded: true,
		cursorMemory:      make(map[string]int),
		itemCache:         make(map[string][]model.Item),
		selectedItems:     make(map[string]bool),
		selectionAnchor:   -1,
	}

	// Namespace state.
	if st.AllNamespaces {
		tab.allNamespaces = true
	} else if st.Namespace != "" {
		tab.namespace = st.Namespace
		if len(st.SelectedNamespaces) > 0 {
			tab.selectedNamespaces = make(map[string]bool, len(st.SelectedNamespaces))
			for _, ns := range st.SelectedNamespaces {
				tab.selectedNamespaces[ns] = true
			}
		} else {
			tab.selectedNamespaces = map[string]bool{st.Namespace: true}
		}
	} else {
		tab.allNamespaces = true
	}

	// Resource type and navigation level.
	if st.ResourceType != "" {
		rt, ok := model.FindResourceType(st.ResourceType)
		if ok {
			tab.nav.ResourceType = rt
			tab.nav.Level = model.LevelResources
			if st.ResourceName != "" {
				tab.nav.ResourceName = st.ResourceName
			}
		} else {
			// Resource type not found; start at resource types level.
			tab.nav.Level = model.LevelResourceTypes
		}
	} else if st.Context != "" {
		tab.nav.Level = model.LevelResourceTypes
	} else {
		tab.nav.Level = model.LevelClusters
	}

	return tab
}

// contextInList returns true if the given context name exists in the items list.
func contextInList(ctx string, items []model.Item) bool {
	for _, item := range items {
		if item.Name == ctx {
			return true
		}
	}
	return false
}

// applySessionNamespaces restores namespace settings from session data onto the model.
func applySessionNamespaces(m *Model, allNS bool, ns string, selectedNS []string) {
	if allNS {
		m.allNamespaces = true
		m.selectedNamespaces = nil
	} else if ns != "" {
		m.namespace = ns
		m.allNamespaces = false
		if len(selectedNS) > 0 {
			m.selectedNamespaces = make(map[string]bool, len(selectedNS))
			for _, n := range selectedNS {
				m.selectedNamespaces[n] = true
			}
		} else {
			m.selectedNamespaces = map[string]bool{ns: true}
		}
	}
}
