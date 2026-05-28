package app

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- Bookmark handlers ---

func bookmarkSlotIsContextAware(slot string) bool {
	return len(slot) != 1 || slot[0] < 'A' || slot[0] > 'Z'
}

type bookmarkTargetKind int

const (
	bookmarkTargetCurrent bookmarkTargetKind = iota
	bookmarkTargetContext
	bookmarkTargetUnionSet
)

type bookmarkTarget struct {
	kind                bookmarkTargetKind
	context             string
	lookupContext       string
	unionSetName        string
	unionContexts       []string
	unionContextColors  map[string]string
	unionNamespace      string
	unionStartedFromRow bool
}

func bookmarkSingleNamespace(bm model.Bookmark) string {
	if len(bm.Namespaces) == 1 {
		return strings.TrimSpace(bm.Namespaces[0])
	}
	if len(bm.Namespaces) > 1 {
		return ""
	}
	return strings.TrimSpace(bm.Namespace)
}

func (m Model) currentSingleNamespace() string {
	if m.allNamespaces || len(m.selectedNamespaces) > 1 {
		return ""
	}
	if len(m.selectedNamespaces) == 1 {
		for ns := range m.selectedNamespaces {
			return strings.TrimSpace(ns)
		}
	}
	return strings.TrimSpace(m.namespace)
}

func (m Model) namespaceForUnionBookmark(bm model.Bookmark, set ui.UnionSetConfig, loadSaved bool) (string, bool) {
	if loadSaved {
		ns := bookmarkSingleNamespace(bm)
		return ns, ns != ""
	}
	if ns := m.currentSingleNamespace(); ns != "" {
		return ns, true
	}
	var namespaceLookup func(string) (string, bool)
	if m.client != nil {
		namespaceLookup = m.client.ContextNamespace
	}
	_, resolvedNamespace, _, err := ExpandUnionSetConfig(set, namespaceLookup)
	if err != nil {
		return "", false
	}
	if ns := strings.TrimSpace(resolvedNamespace); ns != "" {
		return ns, true
	}
	if ns := bookmarkSingleNamespace(bm); ns != "" {
		return ns, true
	}
	return "", false
}

func (m Model) resolveBookmarkTarget(bm model.Bookmark, loadSavedNamespace bool) (bookmarkTarget, string, bool) {
	if bm.Context != "" && bm.UnionSet != "" {
		return bookmarkTarget{}, "Bookmark target is invalid", false
	}

	if bm.UnionSet != "" {
		set, ok := m.findUnionSetConfig(bm.UnionSet)
		if !ok {
			return bookmarkTarget{}, fmt.Sprintf("Union set not found: %s", bm.UnionSet), false
		}
		var namespaceLookup func(string) (string, bool)
		if m.client != nil {
			namespaceLookup = m.client.ContextNamespace
		}
		contexts, _, colors, err := ExpandUnionSetConfig(set, namespaceLookup)
		if err != nil {
			return bookmarkTarget{}, err.Error(), false
		}
		if len(contexts) == 0 {
			return bookmarkTarget{}, fmt.Sprintf("Union set has no contexts: %s", bm.UnionSet), false
		}
		ns, ok := m.namespaceForUnionBookmark(bm, set, loadSavedNamespace)
		if !ok {
			return bookmarkTarget{}, "Union-set bookmark requires exactly one namespace", false
		}
		opts := StartupOptions{
			UnionSet:           set.Name,
			UnionContexts:      contexts,
			UnionContextColors: colors,
			Namespaces:         []string{ns},
		}
		var contextExists func(string) bool
		if m.client != nil {
			contextExists = m.client.ContextExists
		}
		if err := ValidateUnionOptions(opts, contextExists); err != nil {
			return bookmarkTarget{}, err.Error(), false
		}
		return bookmarkTarget{
			kind:                bookmarkTargetUnionSet,
			context:             UnionContextSentinel,
			lookupContext:       contexts[0],
			unionSetName:        set.Name,
			unionContexts:       contexts,
			unionContextColors:  colors,
			unionNamespace:      ns,
			unionStartedFromRow: true,
		}, "", true
	}

	if bm.Context != "" {
		return bookmarkTarget{
			kind:          bookmarkTargetContext,
			context:       bm.Context,
			lookupContext: bm.Context,
		}, "", true
	}

	target := bookmarkTarget{
		kind:          bookmarkTargetCurrent,
		context:       m.nav.Context,
		lookupContext: m.nav.Context,
	}
	if m.isUnionSentinel() {
		if len(m.unionContexts) == 0 {
			return bookmarkTarget{}, "Union view has no member contexts", false
		}
		target.lookupContext = m.unionContexts[0]
		target.unionSetName = m.unionSetName
		target.unionContexts = append([]string(nil), m.unionContexts...)
		target.unionContextColors = copyMapStringString(m.unionContextColors)
		target.unionStartedFromRow = m.unionStartedFromPicker
	}
	return target, "", true
}

// bookmarkToSlot saves the current location as a named mark in the given slot.
// Lowercase (a-z) and digit (0-9) slots create context-aware bookmarks that
// remember the current kube context or named union set. Uppercase (A-Z)
// slots create context-free bookmarks that use whatever target is active on
// jump.
// If a bookmark already exists in that slot, it prompts for confirmation.
func (m Model) bookmarkToSlot(slot string) (tea.Model, tea.Cmd) {
	if m.nav.Level < model.LevelResourceTypes {
		m.setStatusMessage("Navigate to a resource type first", true)
		return m, scheduleStatusClear()
	}
	// Lowercase (a-z) and digit (0-9) slots create context-aware bookmarks
	// that remember the current kube context or named union set. Uppercase
	// (A-Z) slots create context-free bookmarks that use whatever target is
	// active on jump.
	isContextAware := bookmarkSlotIsContextAware(slot)
	if m.isUnionSentinel() && isContextAware && m.unionSetName == "" {
		m.setStatusMessage("Context-aware union bookmarks require a named union set", true)
		return m, scheduleStatusClear()
	}
	if m.isUnionSentinel() && isContextAware {
		if _, ok := m.findUnionSetConfig(m.unionSetName); !ok {
			m.setStatusMessage(fmt.Sprintf("Union set not found: %s", m.unionSetName), true)
			return m, scheduleStatusClear()
		}
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
		discoveryCtx := m.nav.Context
		if m.isUnionSentinel() && len(m.unionContexts) > 0 {
			discoveryCtx = m.unionContexts[0]
		}
		resolved, ok := model.FindResourceTypeIn(sel.Extra, m.discoveredResources[discoveryCtx])
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
	if isUnionDashboardResourceKind(rt.Kind) {
		m.setStatusMessage("Select a Kubernetes resource type to bookmark", true)
		return m, scheduleStatusClear()
	}

	// Context-aware bookmarks include the target in the display name and
	// save it for cross-cluster navigation. Context-free bookmarks do not.
	var parts []string
	if isContextAware {
		if m.isUnionSentinel() {
			parts = append(parts, m.unionSetName)
		} else {
			parts = append(parts, m.nav.Context)
		}
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
	bmUnionSet := ""
	if isContextAware {
		if m.isUnionSentinel() {
			bmUnionSet = m.unionSetName
		} else {
			bmContext = m.nav.Context
		}
	}

	bm := model.Bookmark{
		Name:               name,
		Context:            bmContext,
		UnionSet:           bmUnionSet,
		Namespace:          ns,
		Namespaces:         nsList,
		NsSelectionNegated: m.nsSelectionNegated,
		ResourceType:       rt.ResourceRef(),
		ResourceName:       m.nav.ResourceName,
		Slot:               slot,
	}

	// Check if slot is already in use; if so, ask for confirmation.
	for _, b := range m.bookmarks {
		if b.Slot == slot {
			m.pendingBookmark = &bm
			m.setStatusMessage(fmt.Sprintf("Mark '%s' exists (%s). Overwrite? (Enter/Esc)", slot, b.Name), true)
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
	if err := saveBookmarks(m.bookmarks); err != nil {
		logger.Error("Failed to persist bookmarks after delete", "error", err, "slot", target.Slot)
	}
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
	if err := saveBookmarks(m.bookmarks); err != nil {
		logger.Error("Failed to persist bookmarks after bulk delete", "error", err, "removed", len(filtered))
	}
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
			m.setStatusMessage(fmt.Sprintf("Delete mark %s? (Enter/Esc)", label), true)
		}
		return m, nil
	case "alt+x":
		// Delete-all. Moved off of ctrl+x (now single delete). "cut one"
		// (ctrl+x) vs "cut all" (alt+x) keeps the two hotkeys related.
		if len(filtered) > 0 {
			count := len(filtered)
			m.bookmarkSearchMode = bookmarkModeConfirmDeleteAll
			m.setStatusMessage(fmt.Sprintf("Delete %d bookmark(s)? (Enter/Esc)", count), true)
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

// handleBookmarkConfirmDelete handles Enter/y / Esc/n confirmation for single bookmark deletion.
func (m Model) handleBookmarkConfirmDelete(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.bookmarkSearchMode = bookmarkModeNormal
	switch msg.String() {
	case "enter", "y", "Y":
		cmd := m.bookmarkDeleteCurrent()
		return m, cmd
	default:
		m.setStatusMessage("Cancelled", false)
		return m, scheduleStatusClear()
	}
}

// handleBookmarkConfirmDeleteAll handles Enter/y / Esc/n confirmation for deleting all bookmarks.
func (m Model) handleBookmarkConfirmDeleteAll(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	m.bookmarkSearchMode = bookmarkModeNormal
	switch msg.String() {
	case "enter", "y", "Y":
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

	target, msg, ok := m.resolveBookmarkTarget(bm, m.bookmarkLoadNamespace)
	if !ok {
		m.setStatusMessage(msg, true)
		return m, scheduleStatusClear()
	}

	rt, ok := model.FindResourceTypeIn(bm.ResourceType, m.discoveredResources[target.lookupContext])
	if !ok {
		// Distinguish "discovery hasn't run yet" (key absent from the map)
		// from "discovered, type genuinely not in cluster" (key present, type
		// missing). The former is the session-restore case for CRD-backed
		// bookmarks: stash the bookmark and let updateAPIResourceDiscovery
		// replay it once the discovery message arrives. Without this, popping
		// a bookmark for an ArgoCD Application right after launching lfk
		// failed outright until the user navigated out and back to force a
		// resource-types refresh.
		if _, discovered := m.discoveredResources[target.lookupContext]; !discovered {
			bmCopy := bm
			m.bookmarkAwaitingDiscovery = &bmCopy
			m.setStatusMessage("Discovering resources...", false)
			cmds := []tea.Cmd{scheduleStatusClear()}
			if m.shouldFireDiscoveryFor(target.lookupContext) {
				m.markDiscoveryStarted(target.lookupContext)
				cmds = append(cmds, m.discoverAPIResources(target.lookupContext))
			}
			return m, tea.Batch(cmds...)
		}
		m.setStatusMessage("Resource type not found in current cluster", true)
		return m, scheduleStatusClear()
	}

	// Target resolved successfully. Record the origin so jump_back can return
	// here after this teleport — done before any nav state is mutated, and
	// only on a confirmed-resolvable jump so failed jumps leave no history.
	m.pushJumpHistory()

	// Switch target: context bookmarks exit union mode, named union-set
	// bookmarks activate union mode, and context-free bookmarks preserve the
	// current mode.
	oldCtx := m.nav.Context
	m.applyBookmarkContextSwitch(target)
	m.dashboardPreview = ""
	m.dashboardEventsPreview = ""
	m.monitoringPreview = ""
	m.applyPinnedGroups()

	m.applyBookmarkNamespace(bm, target, oldCtx)

	// Navigate to resource type level first, then optionally deeper.
	m.nav.ResourceType = rt
	m.applyResourceTypeSortDefault(m.nav.ResourceType, m.nav.Context)
	m.nav.ResourceName = bm.ResourceName

	// Navigate to resources level (optionally with a specific resource selected).
	m.nav.Level = model.LevelResources

	// Reset navigation state that doesn't apply.
	m.nav.OwnedName = ""
	m.nav.Namespace = ""

	// Reset column data and history; we'll rebuild from the target level.
	m.leftItems = nil
	m.leftItemsHistory = nil
	m.setMiddleItems(nil)
	m.rightItems = nil
	m.clearRight()

	resourceTypes := m.rebuildLeftHistoryForBookmark(target)

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
			m.cursorMemory[target.context] = i
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
