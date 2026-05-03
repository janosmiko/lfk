package app

import (
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

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
			// Paste counts as an edit: leave history-browse so a
			// follow-up Down doesn't keep navigating history.
			m.queryHistory.leaveBrowse()
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
			m.queryHistory.leaveBrowse()
		}
		return m, nil
	case "ctrl+w":
		m.searchInput.DeleteWord()
		m.jumpToSearchMatch(0)
		m.queryHistory.leaveBrowse()
		return m, nil
	case "ctrl+u":
		m.searchInput.DeleteLine()
		m.jumpToSearchMatch(0)
		m.queryHistory.leaveBrowse()
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
			m.queryHistory.leaveBrowse()
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
	for _, mi := range slices.Backward(matches) {
		if mi <= startIdx {
			m.setCursor(mi)
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
	for _, mi := range slices.Backward(matches) {
		if mi <= start {
			return mi
		}
	}
	return matches[len(matches)-1]
}
