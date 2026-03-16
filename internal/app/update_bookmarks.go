package app

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
)

// --- Bookmark handlers ---

// bookmarkCurrentLocation saves the current navigation state as a bookmark.
func (m Model) bookmarkCurrentLocation() (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Navigate to a resource type first", true)
		return m, scheduleStatusClear()
	}

	// Build a readable name for the bookmark.
	parts := []string{m.nav.Context}
	if m.nav.ResourceType.DisplayName != "" {
		parts = append(parts, m.nav.ResourceType.DisplayName)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	name := strings.Join(parts, " > ")

	ns := m.namespace
	if m.allNamespaces {
		ns = ""
	}

	bm := model.Bookmark{
		Name:         name,
		Context:      m.nav.Context,
		Namespace:    ns,
		ResourceType: m.nav.ResourceType.ResourceRef(),
		ResourceName: m.nav.ResourceName,
	}

	m.bookmarks = addBookmark(m.bookmarks, bm)
	if err := saveBookmarks(m.bookmarks); err != nil {
		m.setStatusMessage("Failed to save bookmark: "+err.Error(), true)
		return m, scheduleStatusClear()
	}
	m.setStatusMessage("Bookmarked: "+name, false)
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
		if bm.Name == target.Name && bm.Context == target.Context &&
			bm.ResourceType == target.ResourceType && bm.ResourceName == target.ResourceName {
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
			filterSet[bm.Name+"\x00"+bm.Context+"\x00"+bm.ResourceType+"\x00"+bm.ResourceName] = true
		}
		var remaining []model.Bookmark
		for _, bm := range m.bookmarks {
			key := bm.Name + "\x00" + bm.Context + "\x00" + bm.ResourceType + "\x00" + bm.ResourceName
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
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0]-'0') - 1
		if idx < len(filtered) {
			return m.navigateToBookmark(filtered[idx])
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
	case "d":
		cmd := m.bookmarkDeleteCurrent()
		return m, cmd
	case "D":
		cmd := m.bookmarkDeleteAll()
		return m, cmd
	case "ctrl+c":
		return m.closeTabOrQuit()
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

// navigateToBookmark jumps to the location described by a bookmark.
func (m Model) navigateToBookmark(bm model.Bookmark) (tea.Model, tea.Cmd) {
	m.overlay = overlayNone
	m.bookmarkFilter.Clear()

	rt, ok := model.FindResourceTypeIn(bm.ResourceType, m.discoveredCRDs[bm.Context])
	if !ok {
		m.setStatusMessage("Unknown resource type in bookmark", true)
		return m, scheduleStatusClear()
	}

	// Switch context.
	m.nav.Context = bm.Context
	m.applyPinnedGroups()

	// Set namespace.
	if bm.Namespace == "" {
		m.allNamespaces = true
	} else {
		m.allNamespaces = false
		m.namespace = bm.Namespace
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
	if crds := m.discoveredCRDs[bm.Context]; len(crds) > 0 {
		resourceTypes = model.MergeWithCRDs(crds)
	} else {
		resourceTypes = model.FlattenedResourceTypes()
	}

	// Set up history: at LevelResources, leftItemsHistory has [contexts], leftItems = resourceTypes.
	m.leftItemsHistory = [][]model.Item{contexts}
	m.leftItems = resourceTypes

	// Reset cursors.
	m.cursors = [5]int{}
	m.cursorMemory = make(map[string]int)
	m.itemCache = make(map[string][]model.Item)
	m.setCursor(0)

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
func (m Model) restoreSession(contexts []model.Item) (tea.Model, tea.Cmd) {
	sess := m.pendingSession
	m.pendingSession = nil
	m.sessionRestored = true

	// Verify the saved context still exists in the loaded context list.
	contextExists := false
	for _, item := range contexts {
		if item.Name == sess.Context {
			contextExists = true
			break
		}
	}
	if !contextExists {
		// Context no longer exists; fall back to normal startup.
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
	if sess.AllNamespaces {
		m.allNamespaces = true
		m.selectedNamespaces = nil
	} else if sess.Namespace != "" {
		m.namespace = sess.Namespace
		m.allNamespaces = false
		if len(sess.SelectedNamespaces) > 0 {
			m.selectedNamespaces = make(map[string]bool, len(sess.SelectedNamespaces))
			for _, ns := range sess.SelectedNamespaces {
				m.selectedNamespaces[ns] = true
			}
		} else {
			m.selectedNamespaces = map[string]bool{sess.Namespace: true}
		}
	}

	cmds := []tea.Cmd{m.discoverCRDs(sess.Context)}

	// If a resource type was saved, navigate deeper.
	if sess.ResourceType != "" {
		rt, ok := model.FindResourceType(sess.ResourceType)
		if ok {
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
