package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpFilterActive {
		return m.handleHelpFilterInput(msg)
	}
	if m.helpSearchActive {
		return m.handleHelpSearchInput(msg)
	}

	switch msg.String() {
	case "esc":
		// Esc cascades: clear search highlights → clear filter → close.
		// Lets the user back out of search/filter state without losing
		// their place on the help screen (close-on-first-Esc would
		// require navigating back from scratch).
		switch {
		case m.helpSearchQuery != "":
			m.helpSearchQuery = ""
			m.helpMatchLines = nil
			m.helpMatchIdx = 0
			return m, nil
		case m.helpFilter.Value != "":
			m.helpFilter.Clear()
			m.helpScroll = 0
			return m, nil
		default:
			m.mode = m.helpPreviousMode
			return m, nil
		}
	case "q", "?", "f1":
		m.mode = m.helpPreviousMode
		return m, nil
	case "j", "down":
		m.helpScroll++
		m.clampHelpScroll()
		return m, nil
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.helpScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		// Use a sentinel and clamp — clampHelpScroll knows the actual
		// max from the current visible help lines, so the model lands
		// exactly at the bottom instead of parking far past it.
		m.helpScroll = 9999
		m.clampHelpScroll()
		return m, nil
	case "ctrl+d":
		m.helpScroll += m.height / 2
		m.clampHelpScroll()
		return m, nil
	case "ctrl+u":
		m.helpScroll -= m.height / 2
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "ctrl+f", "pgdown":
		m.helpScroll += m.height
		m.clampHelpScroll()
		return m, nil
	case "ctrl+b", "pgup":
		m.helpScroll -= m.height
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "home":
		m.helpScroll = 0
		m.pendingG = false
		return m, nil
	case "end":
		m.helpScroll = 9999
		m.clampHelpScroll()
		return m, nil
	case "/":
		// Open search input. Search highlights matches inline without
		// removing non-matching lines (different from f-filter).
		m.helpSearchActive = true
		m.helpSearchInput.SetValue(m.helpSearchQuery)
		m.helpSearchInput.Focus()
		return m, textinput.Blink
	case "f":
		// Open filter input. Filter narrows the visible help to lines
		// matching the query.
		m.helpFilterActive = true
		m.helpSearchInput.SetValue(m.helpFilter.Value)
		m.helpSearchInput.Focus()
		return m, textinput.Blink
	case "n":
		// Navigate to next search match (after / + Enter).
		m.helpJumpToMatch(1)
		return m, nil
	case "N":
		m.helpJumpToMatch(-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleHelpSearchInput runs while the user is typing in the / search
// input. Updates helpSearchQuery on every keystroke (so highlights
// follow the typed text), supports ctrl+n / ctrl+p to jump between
// matches in real time, Enter to apply, Esc to cancel.
func (m Model) handleHelpSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.helpSearchActive = false
		m.helpSearchQuery = ""
		m.helpMatchLines = nil
		m.helpMatchIdx = 0
		m.helpSearchInput.Blur()
		return m, nil
	case "enter":
		m.helpSearchActive = false
		m.helpSearchInput.Blur()
		// Keep helpSearchQuery so highlights persist and n/N navigate
		// after the input closes.
		return m, nil
	case "ctrl+n":
		m.helpJumpToMatch(1)
		return m, nil
	case "ctrl+p":
		m.helpJumpToMatch(-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		var cmd tea.Cmd
		m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
		m.helpSearchQuery = m.helpSearchInput.Value()
		m.helpRecomputeMatches()
		// Jump to the first match so the user sees the highlight without
		// having to manually scroll. Nothing happens if there are no
		// matches.
		if len(m.helpMatchLines) > 0 {
			m.helpMatchIdx = 0
			m.helpScrollToMatch()
		}
		return m, cmd
	}
}

// handleHelpFilterInput runs while the user is typing in the f filter
// input. Filter narrows visible lines as the user types; Enter applies
// (closes input), Esc clears.
func (m Model) handleHelpFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.helpFilterActive = false
		m.helpFilter.Clear()
		m.helpSearchInput.Blur()
		m.helpScroll = 0
		return m, nil
	case "enter":
		m.helpFilterActive = false
		m.helpSearchInput.Blur()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		var cmd tea.Cmd
		m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
		m.helpFilter.Value = m.helpSearchInput.Value()
		m.helpScroll = 0
		// A filter change can shift line indices, invalidating any prior
		// search match cursor — recompute against the new visible set.
		m.helpRecomputeMatches()
		return m, cmd
	}
}

// helpRecomputeMatches walks the current help lines (after filter is
// applied) and records the indices of lines containing the search
// query. Resets helpMatchIdx when the match set changes.
func (m *Model) helpRecomputeMatches() {
	m.helpMatchLines = nil
	if m.helpSearchQuery == "" {
		m.helpMatchIdx = 0
		return
	}
	lines := ui.BuildHelpLines(m.helpFilter.Value, m.helpContextMode)
	for i, line := range lines {
		if ui.MatchLine(line, m.helpSearchQuery) {
			m.helpMatchLines = append(m.helpMatchLines, i)
		}
	}
	if m.helpMatchIdx >= len(m.helpMatchLines) {
		m.helpMatchIdx = 0
	}
}

// helpJumpToMatch advances the match cursor by delta and scrolls the
// viewport so the new match line is visible. No-op when there are no
// matches.
func (m *Model) helpJumpToMatch(delta int) {
	if len(m.helpMatchLines) == 0 {
		return
	}
	m.helpMatchIdx = (m.helpMatchIdx + delta) % len(m.helpMatchLines)
	if m.helpMatchIdx < 0 {
		m.helpMatchIdx += len(m.helpMatchLines)
	}
	m.helpScrollToMatch()
}

// helpScrollToMatch positions helpScroll so the current match line sits
// roughly in the middle of the visible viewport — gives the user
// surrounding context instead of pinning the match to the top edge.
func (m *Model) helpScrollToMatch() {
	if len(m.helpMatchLines) == 0 {
		return
	}
	target := m.helpMatchLines[m.helpMatchIdx]
	visible := m.helpVisibleLines()
	m.helpScroll = max(target-visible/2, 0)
}

// helpCurrentMatchLine returns the line index in the current
// (post-filter) help line list of the match the n/N cursor sits on,
// or -1 when there is no active search match. The renderer uses this
// to render the current match with the distinct
// SelectedSearchHighlightStyle so the user can tell which match the
// next n/N press will move from.
func (m *Model) helpCurrentMatchLine() int {
	if len(m.helpMatchLines) == 0 || m.helpMatchIdx < 0 || m.helpMatchIdx >= len(m.helpMatchLines) {
		return -1
	}
	return m.helpMatchLines[m.helpMatchIdx]
}

// helpVisibleLines returns the number of help-content rows that fit
// inside the overlay box. Defers to ui.HelpVisibleLines so the app's
// clamp/scroll-to-match math matches the renderer's display clamp
// exactly — when the formulas drift, "↓ more below" never disappears
// because the clamp stops short of the renderer's actual bottom.
//
// Pass the same screen height view.go passes to RenderHelpScreen
// (m.height - 1), not m.height — the bottom row is reserved for the
// status bar.
func (m *Model) helpVisibleLines() int {
	return ui.HelpVisibleLines(m.height - 1)
}

// clampHelpScroll bounds m.helpScroll to [0, max] where max is the
// largest scroll offset that still keeps the last help line in view.
// Without this clamp, G/end set helpScroll to 9999 and ctrl+d past
// the end keeps incrementing — both park the model way past the valid
// range, so subsequent ctrl+u presses spend dozens of keystrokes
// undoing phantom scroll before the viewport visibly moves.
func (m *Model) clampHelpScroll() {
	totalLines := len(ui.BuildHelpLines(m.helpFilter.Value, m.helpContextMode))
	maxScroll := max(totalLines-m.helpVisibleLines(), 0)
	if m.helpScroll > maxScroll {
		m.helpScroll = maxScroll
	}
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		text := strings.TrimRight(string(msg.Runes), "\n")
		if strings.Contains(text, "\n") {
			m.triggerPasteConfirm(text, pasteTargetFilter)
			return m, nil
		}
		if text != "" {
			m.filterInput.Insert(text)
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
		}
		return m, nil
	}
	switch msg.String() {
	case "enter":
		m.filterText = m.filterInput.Value
		m.filterActive = false
		// Keep filterBroadMode as-is: visibleMiddleItems consults it to
		// decide whether to scan column values, so resetting here would
		// silently drop the user's broad-scope filter the moment they
		// confirm. Reset happens on Esc (cancel) or when a new filter
		// input starts via handleKeyFilter.
		m.setCursor(0)
		m.clampCursor()
		m.queryHistory.add(m.filterInput.Value)
		m.queryHistory.save()
		// The cursor now points at the first filter match — a different
		// item than before. Without invalidation the right pane keeps
		// rendering the previous selection's rightItems (and skips the
		// loader), so the user sees stale children for several seconds
		// until the new preview fetch returns. Bumping requestGen also
		// discards any in-flight preview from the pre-filter cursor.
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "esc":
		m.filterActive = false
		m.filterBroadMode = false
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "up":
		m.filterInput.Set(m.queryHistory.up(m.filterInput.Value))
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "down":
		m.filterInput.Set(m.queryHistory.down())
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "tab":
		// Toggle broad mode: also match against column values
		// (annotations, labels, finalizers, CRD printer columns, ...).
		m.filterBroadMode = !m.filterBroadMode
		// Reset cursor since the visible set may change shape.
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "backspace":
		if len(m.filterInput.Value) > 0 {
			m.filterInput.Backspace()
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
			// Editing a recalled entry leaves history navigation: the
			// edited text becomes the new draft, so a later Down past
			// newest restores the edits, not the original pre-recall
			// draft.
			m.queryHistory.reset()
		}
		return m, nil
	case "ctrl+w":
		m.filterInput.DeleteWord()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		m.queryHistory.reset()
		return m, nil
	case "ctrl+u":
		m.filterInput.DeleteLine()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		m.queryHistory.reset()
		return m, nil
	case "ctrl+a":
		m.filterInput.Home()
		return m, nil
	case "ctrl+e":
		m.filterInput.End()
		return m, nil
	case "left":
		m.filterInput.Left()
		return m, nil
	case "right":
		m.filterInput.Right()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.filterInput.Insert(key)
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
			m.queryHistory.reset()
		}
		return m, nil
	}
}

func (m Model) handleSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle paste events.
	if msg.Paste {
		text := strings.TrimRight(string(msg.Runes), "\n")
		if strings.Contains(text, "\n") {
			m.triggerPasteConfirm(text, pasteTargetSearch)
			return m, nil
		}
		if text != "" {
			m.searchInput.Insert(text)
			m.jumpToSearchMatch(0)
		}
		return m, nil
	}
	switch msg.String() {
	case "enter":
		m.searchActive = false
		// Keep searchBroadMode as-is so n/N (jumpToSearchMatch reads
		// this flag) stay in the same scope as the just-confirmed query.
		// Reset on Esc or when a new search starts via handleKeySearch.
		m.queryHistory.add(m.searchInput.Value)
		m.queryHistory.save()
		m.syncExpandedGroup()
		// Confirming the search lands the cursor on a different item than
		// when search started. Invalidate so the right pane drops the
		// stale preview, arms the spinner, and a fresh fetch routes to
		// the new cursor instead of the pre-search one.
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "esc":
		m.searchActive = false
		m.searchBroadMode = false
		m.searchInput.Clear()
		m.setCursor(m.searchPrevCursor)
		m.clampCursor()
		m.syncExpandedGroup()
		// Restoring the cursor to the pre-search position is also a
		// cursor change from the user's last jumpToSearchMatch target,
		// so the preview must invalidate here too.
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "up":
		m.searchInput.Set(m.queryHistory.up(m.searchInput.Value))
		m.jumpToSearchMatch(0)
		return m, nil
	case "down":
		m.searchInput.Set(m.queryHistory.down())
		m.jumpToSearchMatch(0)
		return m, nil
	case "tab":
		// Toggle broad mode: searchMatchesItem also walks column values.
		m.searchBroadMode = !m.searchBroadMode
		m.jumpToSearchMatch(0)
		return m, nil
	case "backspace":
		if len(m.searchInput.Value) > 0 {
			m.searchInput.Backspace()
			m.jumpToSearchMatch(0)
			// Editing a recalled entry leaves history navigation; see
			// the analogous comment in handleFilterKey for rationale.
			m.queryHistory.reset()
		}
		return m, nil
	case "ctrl+w":
		m.searchInput.DeleteWord()
		m.jumpToSearchMatch(0)
		m.queryHistory.reset()
		return m, nil
	case "ctrl+u":
		m.searchInput.DeleteLine()
		m.jumpToSearchMatch(0)
		m.queryHistory.reset()
		return m, nil
	case "ctrl+a":
		m.searchInput.Home()
		return m, nil
	case "ctrl+e":
		m.searchInput.End()
		return m, nil
	case "left":
		m.searchInput.Left()
		return m, nil
	case "right":
		m.searchInput.Right()
		return m, nil
	case "ctrl+n":
		m.jumpToSearchMatch(m.cursor() + 1)
		return m, nil
	case "ctrl+p":
		m.jumpToPrevSearchMatch(m.cursor() - 1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.searchInput.Insert(key)
			m.jumpToSearchMatch(0)
			m.queryHistory.reset()
		}
		return m, nil
	}
}

// expandSearchQuery returns the query and its abbreviation expansion (if any).
func expandSearchQuery(query string) []string {
	queries := []string{query}
	// Check abbreviation with lowercase (abbreviations are case-insensitive).
	q := strings.ToLower(query)
	if expanded, ok := ui.SearchAbbreviations[q]; ok {
		queries = append(queries, expanded)
	}
	return queries
}

func (m *Model) searchMatches(name string, queries []string) bool {
	for _, q := range queries {
		if ui.MatchLine(name, q) {
			return true
		}
	}
	return false
}

// searchMatchIndices returns the items indices that n/N navigation
// should visit.
//
// Default mode: name matches only (and broad-mode column matches when
// on at deeper levels, via searchMatchesItem). Plain `/foo` matches
// resource type names only.
//
// Broad mode (Tab) at LevelResourceTypes additionally pulls in every
// member of any category whose name matches the query — n/N then
// cycles through the whole "matching group" the same way `f`
// expansion does. So Tab + `/argo` cycles Applications and
// ApplicationSets (both under "Argo CD"); Tab + `/ing` unions
// Ingresses/Monitoring (name matches) with every Networking member.
//
// Category expansion is gated on both broad mode AND
// LevelResourceTypes: at deeper levels the category bar isn't
// rendered, so a category-only match would jump n/N to a row with no
// visible highlight; and without broad mode the user is asking for a
// strict name search.
func (m *Model) searchMatchIndices(items []model.Item, queries []string) []int {
	var nameMatches []int
	for i := range items {
		if items[i].Kind == "__collapsed_group__" {
			continue
		}
		if m.searchMatchesItem(items[i], queries) {
			nameMatches = append(nameMatches, i)
		}
	}
	if !m.searchBroadMode || m.nav.Level != model.LevelResourceTypes {
		if len(nameMatches) == 0 {
			return nil
		}
		return nameMatches
	}

	// Broad-mode category expansion: figure out which categories
	// match the query, then union name matches with every member of
	// those categories. Returned indices are sorted and deduplicated.
	matchedCats := make(map[string]bool)
	for i := range items {
		if items[i].Kind == "__collapsed_group__" {
			continue
		}
		cat := items[i].Category
		if cat == "" || matchedCats[cat] {
			continue
		}
		if m.searchMatches(cat, queries) {
			matchedCats[cat] = true
		}
	}
	if len(matchedCats) == 0 {
		if len(nameMatches) == 0 {
			return nil
		}
		return nameMatches
	}
	included := make(map[int]bool, len(nameMatches))
	for _, i := range nameMatches {
		included[i] = true
	}
	for i := range items {
		if items[i].Kind == "__collapsed_group__" {
			continue
		}
		if matchedCats[items[i].Category] {
			included[i] = true
		}
	}
	out := make([]int, 0, len(included))
	for i := range items {
		if included[i] {
			out = append(out, i)
		}
	}
	return out
}

// searchMatchesItem checks if an item matches the search query by
// what's visibly highlighted on screen — the item's name. When
// searchBroadMode is on (Tab toggle inside the search input), also
// scans every visible column value (annotations, labels, finalizers,
// CRD additionalPrinterColumns, custom user columns). Internal-prefix
// columns stay excluded.
//
// Category is intentionally NOT matched here — that lives in
// searchMatchIndices' fallback pass. Counting every item under a
// category-matched bar as a search hit turned n/N into a tour of
// every resource in that group — e.g. "/ing" would step through
// every Networking item because the category name contains "ing".
func (m *Model) searchMatchesItem(item model.Item, queries []string) bool {
	if m.searchMatches(item.Name, queries) {
		return true
	}
	if m.searchBroadMode {
		for _, kv := range item.Columns {
			if isInternalColumnKey(kv.Key) {
				continue
			}
			if m.searchMatches(kv.Value, queries) {
				return true
			}
		}
	}
	return false
}

// isInternalColumnKey identifies column keys that hold render-only
// metadata (deletion timestamps, secret payloads, owner refs, condition
// objects, etc.) rather than text the user would type in a filter. Kept
// in sync with the same exclusion set used by collectExtraToggleEntries
// in update_column_toggle.go and the broad-filter path in
// visibleMiddleItems.
func isInternalColumnKey(key string) bool {
	return strings.HasPrefix(key, "__") ||
		strings.HasPrefix(key, "secret:") ||
		strings.HasPrefix(key, "data:") ||
		strings.HasPrefix(key, "owner:") ||
		strings.HasPrefix(key, "condition:") ||
		strings.HasPrefix(key, "step:") ||
		strings.HasPrefix(key, "cond:")
}

func (m *Model) jumpToSearchMatch(startIdx int) {
	if m.searchInput.Value == "" {
		return
	}
	queries := expandSearchQuery(m.searchInput.Value)

	// At LevelResourceTypes with collapsed groups, search ALL items (not just visible).
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		m.searchAllItems(queries, startIdx, true)
		return
	}

	visible := m.visibleMiddleItems()
	matches := m.searchMatchIndices(visible, queries)
	if len(matches) == 0 {
		return
	}
	for _, mi := range matches {
		if mi >= startIdx {
			m.setCursor(mi)
			return
		}
	}
	m.setCursor(matches[0])
}

func (m *Model) jumpToPrevSearchMatch(startIdx int) {
	if m.searchInput.Value == "" {
		return
	}
	queries := expandSearchQuery(m.searchInput.Value)

	// At LevelResourceTypes with collapsed groups, search ALL items (not just visible).
	if m.nav.Level == model.LevelResourceTypes && !m.allGroupsExpanded {
		m.searchAllItems(queries, startIdx, false)
		return
	}

	visible := m.visibleMiddleItems()
	matches := m.searchMatchIndices(visible, queries)
	if len(matches) == 0 {
		return
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i] <= startIdx {
			m.setCursor(matches[i])
			return
		}
	}
	m.setCursor(matches[len(matches)-1])
}

// searchAllItems searches through ALL middleItems (including collapsed groups)
// and expands the matching group if needed. Used for search at LevelResourceTypes.
func (m *Model) searchAllItems(queries []string, startIdx int, forward bool) {
	// Map startIdx (visible cursor) to the corresponding item in middleItems.
	visible := m.visibleMiddleItems()
	var currentItem model.Item
	if startIdx >= 0 && startIdx < len(visible) {
		currentItem = visible[startIdx]
	}

	// Find the current item's index in the full middleItems list.
	allItems := m.middleItems
	fullStart := 0
	for i, item := range allItems {
		if item.Name == currentItem.Name && item.Kind == currentItem.Kind &&
			item.Category == currentItem.Category && item.Extra == currentItem.Extra {
			fullStart = i
			break
		}
	}

	matchIdx := m.searchAllItemsFind(allItems, queries, fullStart, forward)

	if matchIdx < 0 {
		return
	}

	// Expand the matched item's group if it's currently collapsed.
	matchedItem := allItems[matchIdx]
	if matchedItem.Category != "" && matchedItem.Category != m.expandedGroup {
		m.expandedGroup = matchedItem.Category
	}

	// Find the matched item in the now-visible list and set cursor.
	newVisible := m.visibleMiddleItems()
	for i, item := range newVisible {
		if item.Name == matchedItem.Name && item.Kind == matchedItem.Kind &&
			item.Category == matchedItem.Category && item.Extra == matchedItem.Extra {
			m.setCursor(i)
			return
		}
	}
}

// searchAllItemsFind finds the matching item index in forward or backward direction.
func (m *Model) searchAllItemsFind(allItems []model.Item, queries []string, start int, forward bool) int {
	matches := m.searchMatchIndices(allItems, queries)
	if len(matches) == 0 {
		return -1
	}
	if forward {
		for _, mi := range matches {
			if mi >= start {
				return mi
			}
		}
		return matches[0]
	}
	for i := len(matches) - 1; i >= 0; i-- {
		if matches[i] <= start {
			return matches[i]
		}
	}
	return matches[len(matches)-1]
}

// handleCommandBarKey processes key events when the command bar is active.
func (m Model) handleCommandBarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if msg.Paste {
		return m.handleCommandBarPaste(msg)
	}

	key := msg.String()

	// Right or Space accepts ghost preview if active (not loading placeholders).
	if (key == "right" || key == " ") && m.commandBarPreview != "" && m.commandBarPreview != "loading..." {
		m.commandBarInput.Set(m.commandBarApplySuggestion(m.commandBarPreview))
		m.commandBarPreview = ""
		return m.commandBarRefreshSuggestions()
	}

	// Tab/Shift+Tab/Ctrl+N/P don't clear preview (they cycle).
	// All other keys clear the preview.
	if key != "tab" && key != "shift+tab" && key != "ctrl+n" && key != "ctrl+p" {
		m.commandBarPreview = ""
	}

	switch key {
	case "ctrl+@", "ctrl+space":
		return m.commandBarRefreshSuggestions()
	case "esc":
		return m.commandBarHandleEsc()
	case "ctrl+c":
		return m.commandBarClose(), nil
	case "enter":
		return m.commandBarEnter()
	case "tab", "ctrl+n", "down":
		return m.commandBarCycleForward(key)
	case "shift+tab", "ctrl+p", "up":
		return m.commandBarCycleBackward(key)
	case "ctrl+d", "ctrl+u":
		return m.commandBarScrollSuggestions(key, 5)
	case "ctrl+f", "ctrl+b":
		return m.commandBarScrollSuggestions(key, 10)
	case "ctrl+a":
		m.commandBarInput.Home()
		return m, nil
	case "ctrl+e":
		m.commandBarInput.End()
		return m, nil
	case "backspace":
		if len(m.commandBarInput.Value) > 0 {
			m.commandBarInput.Backspace()
		}
		return m.commandBarRefreshSuggestions()
	case "ctrl+w":
		m.commandBarInput.DeleteWord()
		return m.commandBarRefreshSuggestions()
	case "left":
		m.commandBarInput.Left()
		return m, nil
	case "right":
		m.commandBarInput.Right()
		return m, nil
	default:
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.commandBarInput.Insert(key)
		}
		return m.commandBarRefreshSuggestions()
	}
}

func (m Model) commandBarHandleEsc() (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) > 0 {
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		m.commandBarPreview = ""
		return m, nil
	}
	return m.commandBarClose(), nil
}

func (m Model) commandBarCycleForward(key string) (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) > 0 {
		sel := m.commandBarSuggestions[m.commandBarSelectedSuggestion]
		if sel.Category == "status" {
			return m, nil
		}
		if key == "tab" && m.commandBarActionableSuggestionCount() == 1 {
			m.commandBarInput.Set(m.commandBarApplySuggestion(sel.Text))
			m.commandBarPreview = ""
			return m.commandBarRefreshSuggestions()
		}
		m.commandBarCycleSuggestion(1)
		for m.commandBarSuggestions[m.commandBarSelectedSuggestion].Category == "status" {
			m.commandBarCycleSuggestion(1)
		}
		m.commandBarPreview = m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text
		return m, nil
	}
	if key == "down" {
		m.commandBarInput.Set(m.commandHistory.down())
	}
	return m, nil
}

func (m Model) commandBarCycleBackward(key string) (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) > 0 {
		m.commandBarCycleSuggestion(-1)
		for m.commandBarSuggestions[m.commandBarSelectedSuggestion].Category == "status" {
			m.commandBarCycleSuggestion(-1)
		}
		m.commandBarPreview = m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text
		return m, nil
	}
	if key == "up" {
		m.commandBarInput.Set(m.commandHistory.up(m.commandBarInput.Value))
	}
	return m, nil
}

func (m Model) commandBarScrollSuggestions(key string, amount int) (tea.Model, tea.Cmd) {
	if len(m.commandBarSuggestions) == 0 {
		// ctrl+u without suggestions: delete line.
		if key == "ctrl+u" {
			m.commandBarInput.DeleteLine()
			return m.commandBarRefreshSuggestions()
		}
		return m, nil
	}
	delta := amount
	if key == "ctrl+u" || key == "ctrl+b" {
		delta = -amount
	}
	m.commandBarCycleSuggestion(delta)
	m.commandBarPreview = m.commandBarSuggestions[m.commandBarSelectedSuggestion].Text
	return m, nil
}

// commandBarActionableSuggestionCount returns the number of suggestions
// that are not status placeholders (e.g., "loading...").
func (m Model) commandBarActionableSuggestionCount() int {
	count := 0
	for _, s := range m.commandBarSuggestions {
		if s.Category != "status" {
			count++
		}
	}
	return count
}

func (m Model) handleCommandBarPaste(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	text := strings.TrimRight(string(msg.Runes), "\n")
	if strings.Contains(text, "\n") {
		m.triggerPasteConfirm(text, pasteTargetCommandBar)
		return m, nil
	}
	if text != "" {
		m.commandBarInput.Insert(text)
	}
	return m.commandBarRefreshSuggestions()
}

func (m Model) commandBarClose() Model {
	m.commandBarActive = false
	m.commandBarInput.Clear()
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	m.commandBarPreview = ""
	return m
}

func (m Model) commandBarEnter() (tea.Model, tea.Cmd) {
	// If suggestions are visible, accept the selected one and clear suggestions.
	if len(m.commandBarSuggestions) > 0 {
		suggestion := m.commandBarSuggestions[m.commandBarSelectedSuggestion]
		if suggestion.Category != "status" {
			m.commandBarInput.Set(m.commandBarApplySuggestion(suggestion.Text))
		}
		m.commandBarPreview = ""
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	}

	// No suggestions visible: execute the current input.
	m.commandBarActive = false
	input := m.commandBarInput.Value
	m.commandBarInput.Clear()
	m.commandBarSuggestions = nil
	m.commandBarSelectedSuggestion = 0
	m.commandBarPreview = ""
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return m, nil
	}
	m.commandHistory.add(trimmed)
	m.commandHistory.save()
	return m.executeCommandBarInput(input)
}

func (m *Model) commandBarCycleSuggestion(delta int) {
	if len(m.commandBarSuggestions) == 0 {
		return
	}
	m.commandBarSelectedSuggestion += delta
	n := len(m.commandBarSuggestions)
	m.commandBarSelectedSuggestion = ((m.commandBarSelectedSuggestion % n) + n) % n
}

func (m Model) commandBarRefreshSuggestions() (Model, tea.Cmd) {
	oldLoading := m.commandBarNameLoading
	m.commandBarSuggestions = m.generateCommandBarSuggestions()
	m.commandBarSelectedSuggestion = 0
	m.commandBarPreview = ""

	// If a new async fetch was triggered, add a loading placeholder and fire the fetch.
	if m.commandBarNameLoading != "" && m.commandBarNameLoading != oldLoading {
		m.commandBarSuggestions = append(m.commandBarSuggestions,
			ui.Suggestion{Text: "loading...", Category: "status"})
		// Parse cache key to get namespace and resource type.
		parts := strings.SplitN(m.commandBarNameLoading, "/", 3)
		if len(parts) == 3 {
			return m, m.fetchCommandBarResourceNames(parts[2], parts[1])
		}
	}
	return m, nil
}

// commandBarApplySuggestion replaces the current partial word in the input
// with the accepted suggestion, followed by a trailing space.
func (m Model) commandBarApplySuggestion(suggestion string) string {
	input := m.commandBarInput.Value
	// If input ends with a space, append the suggestion as a new word.
	if strings.HasSuffix(input, " ") || input == "" {
		return input + suggestion
	}
	// Otherwise replace the last partial word.
	if idx := strings.LastIndex(input, " "); idx >= 0 {
		return input[:idx+1] + suggestion
	}
	return suggestion
}
