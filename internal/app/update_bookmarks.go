package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Bookmark handlers ---

// bookmarkToSlot saves the current location as a named mark in the given slot.
// Lowercase (a-z) and digit (0-9) slots create context-aware bookmarks that
// remember the current kube context. Uppercase (A-Z) slots create
// context-free bookmarks that use whatever context is active on jump.
// If a bookmark already exists in that slot, it prompts for confirmation.
func (m Model) bookmarkToSlot(slot string) (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Navigate to a resource type first", true)
		return m, scheduleStatusClear()
	}

	// Resolve which resource type the bookmark refers to. At LevelResources
	// and deeper, m.nav.ResourceType has been populated by navigation. At
	// LevelResourceTypes, nav.ResourceType is still zero — fall back to the
	// item currently under the cursor in the middle column so the user can
	// bookmark a resource type from the sidebar directly.
	rt := m.nav.ResourceType
	if m.nav.Level == model.LevelResourceTypes {
		sel := m.selectedMiddleItem()
		if sel == nil {
			m.setStatusMessage("Navigate to a resource type first", true)
			return m, scheduleStatusClear()
		}
		if sel.Kind == "__collapsed_group__" || sel.Extra == "__overview__" || sel.Extra == "__monitoring__" {
			m.setStatusMessage("Select a resource type to bookmark", true)
			return m, scheduleStatusClear()
		}
		resolved, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[m.nav.Context])
		if !ok {
			// Discovery may not have run yet (seed sidebar). Fall back to
			// the seed set the same way restoreSessionResourceType does.
			resolved, ok = model.FindResourceTypeIn(sel.Extra, model.SeedResources())
		}
		if ok {
			rt = resolved
		} else {
			// Last-resort synthesis from the sidebar item so the bookmark
			// still records the ResourceRef. The jump code will look up the
			// current cluster's discovered set at navigation time.
			rt = model.ResourceTypeEntry{Kind: sel.Kind}
			if parts := strings.SplitN(sel.Extra, "/", 3); len(parts) == 3 {
				rt.APIGroup = parts[0]
				rt.APIVersion = parts[1]
				rt.Resource = parts[2]
			}
		}
	}

	// Lowercase (a-z) and digit (0-9) slots create context-aware bookmarks
	// that remember the current kube context. Uppercase (A-Z) slots create
	// context-free bookmarks that use whatever context is active on jump.
	isContextAware := len(slot) != 1 || slot[0] < 'A' || slot[0] > 'Z'

	// Context-aware bookmarks include the context in the display name and
	// save it for cross-cluster navigation. Context-free bookmarks do not.
	var parts []string
	if isContextAware {
		parts = append(parts, m.nav.Context)
	}
	if label := model.DisplayNameFor(rt); label != "" {
		parts = append(parts, label)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	name := strings.Join(parts, " > ")

	// Always capture the current namespace scope so it's available for
	// an opt-in override at jump time (Tab in the bookmark overlay).
	// Context-free slots still ignore it on a default jump — the slot
	// case controls defaults, not persistence.
	var ns string
	var nsList []string
	switch {
	case m.allNamespaces:
		ns = ""
	case len(m.selectedNamespaces) > 1:
		nsList = make([]string, 0, len(m.selectedNamespaces))
		for n := range m.selectedNamespaces {
			nsList = append(nsList, n)
		}
		sort.Strings(nsList)
		ns = nsList[0] // primary namespace for backward compat display
	default:
		ns = m.namespace
	}

	bmContext := ""
	if isContextAware {
		bmContext = m.nav.Context
	}

	bm := model.Bookmark{
		Name:         name,
		Context:      bmContext,
		Namespace:    ns,
		Namespaces:   nsList,
		ResourceType: rt.ResourceRef(),
		ResourceName: m.nav.ResourceName,
		Slot:         slot,
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
// Creates a new slice to avoid mutating the original backing array.
func (m Model) saveBookmark(bm model.Bookmark) (tea.Model, tea.Cmd) {
	newBookmarks := make([]model.Bookmark, 0, len(m.bookmarks)+1)
	for _, b := range m.bookmarks {
		if b.Slot != bm.Slot {
			newBookmarks = append(newBookmarks, b)
		}
	}
	newBookmarks = append(newBookmarks, bm)
	m.bookmarks = newBookmarks

	if err := saveBookmarks(m.bookmarks); err != nil {
		m.setStatusMessage("Failed to save mark: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	kind := "context-free"
	if bm.IsContextAware() {
		kind = "context-aware"
	}
	m.setStatusMessage(
		fmt.Sprintf("Mark '%s' set: %s (%s)", bm.Slot, bm.Name, kind),
		false,
	)
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
// Always returns a new slice to prevent callers from aliasing m.bookmarks.
func (m *Model) filteredBookmarks() []model.Bookmark {
	if m.bookmarkFilter.Value == "" {
		return append([]model.Bookmark(nil), m.bookmarks...)
	}
	rawQuery := m.bookmarkFilter.Value
	var filtered []model.Bookmark
	for _, bm := range m.bookmarks {
		if ui.MatchLine(bm.Name, rawQuery) {
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
	m.overlayCursor = clampOverlayCursor(m.overlayCursor, 0, len(newFiltered)-1)
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
	// Handle navigation/scroll keys.
	if ret, ok := m.handleBookmarkNavKey(msg, filtered); ok {
		return ret, nil
	}
	// Handle action keys.
	switch msg.String() {
	case "esc":
		m.overlay = overlayNone
		m.bookmarkFilter.Clear()
		m.bookmarkSearchMode = bookmarkModeNormal
		// Discard any pending "load namespace" flag so the next open
		// starts from the default (don't load).
		m.bookmarkLoadNamespace = false
		return m, nil
	case "tab":
		// Arm the "load saved namespace" flag for the next jump. The
		// title bar picks this up via the [LOAD NAMESPACE] chip so
		// the user sees what Enter / slot-key will do.
		m.bookmarkLoadNamespace = !m.bookmarkLoadNamespace
		return m, nil
	case "enter":
		if len(filtered) > 0 && m.overlayCursor >= 0 && m.overlayCursor < len(filtered) {
			return m.navigateToBookmark(filtered[m.overlayCursor])
		}
		return m, nil
	case "/":
		m.bookmarkSearchMode = bookmarkModeFilter
		m.bookmarkFilter.Clear()
		return m, nil
	case "ctrl+x":
		// Single-bookmark delete. Moved off of uppercase "D" so that slot
		// can be used to jump to context-free bookmarks stored in slot D.
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
	case "alt+x":
		// Delete-all. Moved off of ctrl+x (now single delete). "cut one"
		// (ctrl+x) vs "cut all" (alt+x) keeps the two hotkeys related.
		if len(filtered) > 0 {
			count := len(filtered)
			m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
			m.setStatusMessage(fmt.Sprintf("Delete %d bookmark(s)? (y/n)", count), true)
		}
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && ((key[0] >= 'a' && key[0] <= 'z') || (key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= '0' && key[0] <= '9')) {
			return m.jumpToSlot(key)
		}
	}
	return m, nil
}

// handleBookmarkNavKey handles cursor navigation keys in the bookmark overlay.
func (m Model) handleBookmarkNavKey(msg tea.KeyMsg, filtered []model.Bookmark) (Model, bool) {
	maxIdx := len(filtered) - 1
	switch msg.String() {
	case "j", "down", "ctrl+n":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 1, maxIdx)
		return m, true
	case "k", "up", "ctrl+p":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -1, maxIdx)
		return m, true
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.overlayCursor = 0
			return m, true
		}
		m.pendingG = true
		return m, true
	case "G":
		if len(filtered) > 0 {
			m.overlayCursor = maxIdx
		}
		return m, true
	case "ctrl+d":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 10, maxIdx)
		return m, true
	case "ctrl+u":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -10, maxIdx)
		return m, true
	case "ctrl+f", "pgdown":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, 20, maxIdx)
		return m, true
	case "ctrl+b", "pgup":
		m.overlayCursor = clampOverlayCursor(m.overlayCursor, -20, maxIdx)
		return m, true
	case "home":
		m.pendingG = false
		m.overlayCursor = 0
		return m, true
	case "end":
		m.pendingG = false
		if len(filtered) > 0 {
			m.overlayCursor = maxIdx
		}
		return m, true
	}
	return m, false
}

// handleBookmarkFilterMode handles keys when the bookmark overlay is in filter input mode.
func (m Model) handleBookmarkFilterMode(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		switch handlePastedText(&m.bookmarkFilter, msg.Runes) {
		case filterContinue:
			m.overlayCursor = 0
			return m, nil
		case filterPasteMultiline:
			m.triggerPasteConfirm(strings.TrimRight(string(msg.Runes), "\n"), pasteTargetBookmarkFilter)
			return m, nil
		}
		return m, nil
	}
	switch handleFilterKey(&m.bookmarkFilter, msg.String()) {
	case filterEscape:
		m.bookmarkSearchMode = bookmarkModeNormal
		m.bookmarkFilter.Clear()
		m.overlayCursor = 0
		return m, nil
	case filterAccept:
		m.bookmarkSearchMode = bookmarkModeNormal
		return m, nil
	case filterClose:
		return m.closeTabOrQuit()
	case filterContinue:
		m.overlayCursor = 0
		return m, nil
	}
	return m, nil
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

	// For context-free bookmarks, stay in the current cluster context.
	// For context-aware bookmarks, switch to the bookmark's saved context.
	effectiveContext := bm.Context
	if !bm.IsContextAware() {
		effectiveContext = m.nav.Context
	}

	rt, ok := model.FindResourceTypeIn(bm.ResourceType, m.discoveredResources[effectiveContext])
	if !ok {
		m.setStatusMessage("Resource type not found in current cluster", true)
		return m, scheduleStatusClear()
	}

	// Switch context (context-aware bookmarks change cluster, context-free bookmarks keep current).
	m.nav.Context = effectiveContext
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedGroups()

	// Apply the bookmark's saved namespace only when the user asked
	// for it via Tab in the overlay. Default behaviour is "jump to
	// the resource type in my current namespace scope", regardless of
	// slot case; the saved namespace stays in the record so the
	// override can replay it on demand. Consume the flag right after
	// reading so it can't leak into the next open.
	applyNs := m.bookmarkLoadNamespace
	m.bookmarkLoadNamespace = false

	if applyNs {
		switch {
		case bm.Namespace == "" && len(bm.Namespaces) == 0:
			m.allNamespaces = true
			m.selectedNamespaces = nil
		case len(bm.Namespaces) > 1:
			m.allNamespaces = false
			m.namespace = bm.Namespaces[0]
			m.selectedNamespaces = make(map[string]bool, len(bm.Namespaces))
			for _, ns := range bm.Namespaces {
				m.selectedNamespaces[ns] = true
			}
		default:
			m.allNamespaces = false
			ns := bm.Namespace
			if len(bm.Namespaces) == 1 {
				ns = bm.Namespaces[0]
			}
			m.namespace = ns
			m.selectedNamespaces = map[string]bool{ns: true}
		}
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
	if discovered := m.discoveredResources[effectiveContext]; len(discovered) > 0 {
		resourceTypes = model.BuildSidebarItems(discovered)
	} else {
		resourceTypes = model.BuildSidebarItems(model.SeedResources())
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
	cmds := []tea.Cmd{m.loadResources(false), scheduleStatusClear()}
	if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return m, tea.Batch(cmds...)
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

	// Save the clusters-level cursor so navigating back restores it.
	for i, ctx := range contexts {
		if ctx.Name == sess.Context {
			m.cursorMemory[""] = i
			break
		}
	}

	// Navigate into the saved context (same as navigateChild at LevelClusters).
	m.nav.Context = sess.Context
	m.applyPinnedGroups()
	m.nav.Level = model.LevelResourceTypes

	// Re-register security sources against the restored context so source
	// clients point at the right cluster (NewModel wired them to whatever
	// was the current kubeconfig context at startup, which is not
	// necessarily the context the session asks to restore). This mirrors
	// the navigateChildCluster path the user would have taken if they had
	// picked the cluster manually.
	m.refreshSecuritySources()

	// Set up left column history: contexts list becomes the breadcrumb.
	m.leftItemsHistory = nil
	m.leftItems = contexts

	// Load resource types for the middle column. If discovery has already
	// completed for this context, use the full list; otherwise show a loader
	// and let updateAPIResourceDiscovery populate it when ready.
	if discovered, ok := m.discoveredResources[sess.Context]; ok && len(discovered) > 0 {
		m.middleItems = model.BuildSidebarItems(discovered)
	} else {
		m.middleItems = model.BuildSidebarItems(model.SeedResources())
	}
	m.itemCache[m.navKey()] = m.middleItems
	m.clearRight()

	// Restore namespace settings from session.
	applySessionNamespaces(&m, sess.AllNamespaces, sess.Namespace, sess.SelectedNamespaces)

	var cmds []tea.Cmd
	needsDiscovery := false
	if _, ok := m.discoveredResources[sess.Context]; !ok {
		needsDiscovery = true
		cmds = append(cmds, m.discoverAPIResources(sess.Context))
	}
	// Dispatch the security availability probe so the Security category
	// populates itself on cold start.
	if cmd := m.loadSecurityAvailability(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := m.ensureNamespaceCacheFresh(); cmd != nil {
		cmds = append(cmds, cmd)
	}

	// If a resource type was saved, navigate deeper.
	if sess.ResourceType != "" {
		// Runtime discovery is async, so discoveredResources[ctx] is empty
		// at restore time. Fall back to the seed list so core K8s resources
		// (Pods, Deployments, Services, etc.) still resolve and the user
		// lands back on their saved view instead of the resource types list.
		rt, ok := resolveSessionResourceType(sess.ResourceType, m.discoveredResources[sess.Context])
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
	// Show loader while discovery is in-flight.
	if needsDiscovery {
		m.middleItems = nil
		m.loading = true
	}
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
		tab := buildSessionTabState(&st, m.discoveredResources[st.Context])
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
// when the tab becomes active (needsLoad). The discovered parameter
// provides the resource types known for the tab's context so the saved
// ResourceType can be resolved to a concrete GVR.
func buildSessionTabState(st *SessionTab, discovered []model.ResourceTypeEntry) TabState {
	tab := TabState{
		nav: model.NavigationState{
			Context: st.Context,
		},
		splitPreview:      true,
		watchMode:         true,
		warningEventsOnly: true,
		eventGrouping:     true,
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
		rt, ok := resolveSessionResourceType(st.ResourceType, discovered)
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

// resolveSessionResourceType looks up a saved resource type reference
// ("group/version/resource") first in the discovered resource set and then
// in the seed list. The seed fallback is important on session restore
// because runtime API discovery runs asynchronously and has not populated
// discoveredResources yet when the session is first replayed.
func resolveSessionResourceType(ref string, discovered []model.ResourceTypeEntry) (model.ResourceTypeEntry, bool) {
	if rt, ok := model.FindResourceTypeIn(ref, discovered); ok {
		return rt, true
	}
	return model.FindResourceTypeIn(ref, model.SeedResources())
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
