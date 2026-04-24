package app

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpSearchActive {
		switch msg.String() {
		case "esc":
			m.helpSearchActive = false
			m.helpFilter.Clear()
			m.helpSearchInput.Blur()
			return m, nil
		case "enter":
			m.helpSearchActive = false
			m.helpSearchInput.Blur()
			// Keep the filter value active.
			return m, nil
		case "ctrl+c":
			return m.closeTabOrQuit()
		default:
			var cmd tea.Cmd
			m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
			m.helpFilter.Value = m.helpSearchInput.Value()
			m.helpScroll = 0
			return m, cmd
		}
	}

	switch msg.String() {
	case "q", "esc", "?", "f1":
		m.mode = m.helpPreviousMode
		return m, nil
	case "j", "down":
		m.helpScroll++
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
		m.helpScroll = 9999
		return m, nil
	case "ctrl+d":
		m.helpScroll += m.height / 2
		return m, nil
	case "ctrl+u":
		m.helpScroll -= m.height / 2
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "ctrl+f", "pgdown":
		m.helpScroll += m.height
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
		return m, nil
	case "/":
		m.helpSearchActive = true
		m.helpSearchInput.SetValue(m.helpFilter.Value)
		m.helpSearchInput.Focus()
		return m, textinput.Blink
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
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
		m.setCursor(0)
		m.clampCursor()
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
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "backspace":
		if len(m.filterInput.Value) > 0 {
			m.filterInput.Backspace()
			m.filterText = m.filterInput.Value
			m.setCursor(0)
			m.clampCursor()
		}
		return m, nil
	case "ctrl+w":
		m.filterInput.DeleteWord()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
		return m, nil
	case "ctrl+u":
		m.filterInput.DeleteLine()
		m.filterText = m.filterInput.Value
		m.setCursor(0)
		m.clampCursor()
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
		m.syncExpandedGroup()
		// Confirming the search lands the cursor on a different item than
		// when search started. Invalidate so the right pane drops the
		// stale preview, arms the spinner, and a fresh fetch routes to
		// the new cursor instead of the pre-search one.
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "esc":
		m.searchActive = false
		m.searchInput.Clear()
		m.setCursor(m.searchPrevCursor)
		m.clampCursor()
		m.syncExpandedGroup()
		// Restoring the cursor to the pre-search position is also a
		// cursor change from the user's last jumpToSearchMatch target,
		// so the preview must invalidate here too.
		m.invalidatePreviewForCursorChange()
		return m, m.loadPreview()
	case "backspace":
		if len(m.searchInput.Value) > 0 {
			m.searchInput.Backspace()
			m.jumpToSearchMatch(0)
		}
		return m, nil
	case "ctrl+w":
		m.searchInput.DeleteWord()
		m.jumpToSearchMatch(0)
		return m, nil
	case "ctrl+u":
		m.searchInput.DeleteLine()
		m.jumpToSearchMatch(0)
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

// searchMatchesItem checks if an item matches the search query by name or category.
func (m *Model) searchMatchesItem(item model.Item, queries []string) bool {
	if m.searchMatches(item.Name, queries) {
		return true
	}
	if item.Category != "" && m.searchMatches(item.Category, queries) {
		return true
	}
	return false
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
	for i := startIdx; i < len(visible); i++ {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
	for i := 0; i < startIdx && i < len(visible); i++ {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
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
	// Search backwards from startIdx.
	for i := startIdx; i >= 0; i-- {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
	// Wrap around from the end.
	for i := len(visible) - 1; i > startIdx; i-- {
		if m.searchMatchesItem(visible[i], queries) {
			m.setCursor(i)
			return
		}
	}
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
	if forward {
		for i := start; i < len(allItems); i++ {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				return i
			}
		}
		for i := 0; i < start && i < len(allItems); i++ {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				return i
			}
		}
	} else {
		for i := start; i >= 0; i-- {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				return i
			}
		}
		for i := len(allItems) - 1; i > start; i-- {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				return i
			}
		}
	}
	return -1
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
