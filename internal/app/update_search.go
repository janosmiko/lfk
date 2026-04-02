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
	case "ctrl+f":
		m.helpScroll += m.height
		return m, nil
	case "ctrl+b":
		m.helpScroll -= m.height
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
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
	switch msg.String() {
	case "enter":
		m.filterText = m.filterInput.Value
		m.filterActive = false
		m.setCursor(0)
		m.clampCursor()
		return m, m.loadPreview()
	case "esc":
		m.filterActive = false
		m.filterInput.Clear()
		m.filterText = ""
		m.setCursor(0)
		m.clampCursor()
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
	switch msg.String() {
	case "enter":
		m.searchActive = false
		m.syncExpandedGroup()
		return m, m.loadPreview()
	case "esc":
		m.searchActive = false
		m.searchInput.Clear()
		m.setCursor(m.searchPrevCursor)
		m.clampCursor()
		m.syncExpandedGroup()
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
	q := strings.ToLower(query)
	queries := []string{q}
	if expanded, ok := ui.SearchAbbreviations[q]; ok {
		queries = append(queries, expanded)
	}
	return queries
}

func (m *Model) searchMatches(name string, queries []string) bool {
	lower := strings.ToLower(name)
	for _, q := range queries {
		if strings.Contains(lower, q) {
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

	// Search through all items in the specified direction.
	matchIdx := -1
	if forward {
		for i := fullStart; i < len(allItems); i++ {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				matchIdx = i
				break
			}
		}
		if matchIdx < 0 {
			for i := 0; i < fullStart && i < len(allItems); i++ {
				if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
					matchIdx = i
					break
				}
			}
		}
	} else {
		for i := fullStart; i >= 0; i-- {
			if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
				matchIdx = i
				break
			}
		}
		if matchIdx < 0 {
			for i := len(allItems) - 1; i > fullStart; i-- {
				if allItems[i].Kind != "__collapsed_group__" && m.searchMatchesItem(allItems[i], queries) {
					matchIdx = i
					break
				}
			}
		}
	}

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

// handleCommandBarKey processes key events when the command bar is active.
func (m Model) handleCommandBarKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.commandBarActive = false
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "enter":
		m.commandBarActive = false
		input := m.commandBarInput.Value
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		trimmed := strings.TrimSpace(input)
		if trimmed == "" {
			return m, nil
		}
		// Record command in persistent history.
		m.commandHistory.add(trimmed)
		m.commandHistory.save()
		// Handle built-in commands.
		if trimmed == "q" || trimmed == "q!" || trimmed == "quit" {
			return m, tea.Quit
		}
		return m, m.executeCommandBar(input)
	case "up":
		m.commandBarInput.Set(m.commandHistory.up(m.commandBarInput.Value))
		return m, nil
	case "down":
		m.commandBarInput.Set(m.commandHistory.down())
		return m, nil
	case "tab":
		if len(m.commandBarSuggestions) > 0 {
			// Accept the currently selected suggestion into the input.
			suggestion := m.commandBarSuggestions[m.commandBarSelectedSuggestion]
			m.commandBarInput.Set(m.commandBarApplySuggestion(suggestion))
			m.commandBarSuggestions = nil
			m.commandBarSelectedSuggestion = 0
			// Regenerate suggestions for the new input state.
			m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		}
		return m, nil
	case "shift+tab":
		if len(m.commandBarSuggestions) > 0 {
			m.commandBarSelectedSuggestion--
			if m.commandBarSelectedSuggestion < 0 {
				m.commandBarSelectedSuggestion = len(m.commandBarSuggestions) - 1
			}
		}
		return m, nil
	case "right":
		// Cycle forward through suggestions, or move cursor if no suggestions.
		if len(m.commandBarSuggestions) > 0 {
			m.commandBarSelectedSuggestion = (m.commandBarSelectedSuggestion + 1) % len(m.commandBarSuggestions)
		} else {
			m.commandBarInput.Right()
		}
		return m, nil
	case "left":
		// Cycle backward through suggestions, or move cursor if no suggestions.
		if len(m.commandBarSuggestions) > 0 {
			m.commandBarSelectedSuggestion--
			if m.commandBarSelectedSuggestion < 0 {
				m.commandBarSelectedSuggestion = len(m.commandBarSuggestions) - 1
			}
		} else {
			m.commandBarInput.Left()
		}
		return m, nil
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
		m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "ctrl+w":
		// Delete word backwards.
		m.commandBarInput.DeleteWord()
		m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		m.commandBarSelectedSuggestion = 0
		return m, nil
	case "ctrl+c":
		m.commandBarActive = false
		m.commandBarInput.Clear()
		m.commandBarSuggestions = nil
		m.commandBarSelectedSuggestion = 0
		return m, nil
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 && key[0] < 127 {
			m.commandBarInput.Insert(key)
		} else if key == " " {
			m.commandBarInput.Insert(" ")
		}
		m.commandBarSuggestions = m.commandBarGenerateSuggestions()
		m.commandBarSelectedSuggestion = 0
		return m, nil
	}
}

// commandBarApplySuggestion replaces the current partial word in the input
// with the accepted suggestion, followed by a trailing space.
func (m Model) commandBarApplySuggestion(suggestion string) string {
	input := m.commandBarInput.Value
	// If input ends with a space, append the suggestion as a new word.
	if strings.HasSuffix(input, " ") || input == "" {
		return input + suggestion + " "
	}
	// Otherwise replace the last partial word.
	if idx := strings.LastIndex(input, " "); idx >= 0 {
		return input[:idx+1] + suggestion + " "
	}
	return suggestion + " "
}

// commandBarGenerateSuggestions returns contextual suggestions based on the
// current command bar input. It considers the position (which word is being
// typed) and the preceding words to provide relevant completions.
//
// Suggestions are only provided when the input looks like a kubectl command
// (starts with "kubectl " or the first word matches a known subcommand).
// For arbitrary shell commands, no suggestions are returned.
func (m Model) commandBarGenerateSuggestions() []string {
	input := m.commandBarInput.Value

	// Strip optional leading "kubectl " prefix for analysis.
	stripped := input
	hasKubectlPrefix := strings.HasPrefix(strings.ToLower(stripped), "kubectl ")
	if hasKubectlPrefix {
		stripped = stripped[8:]
	}

	// Split into words, keeping track of whether we're mid-word or starting a new one.
	words := strings.Fields(stripped)
	midWord := !strings.HasSuffix(input, " ") && len(input) > 0

	// If the user has typed at least one complete word (or is mid-word on a
	// non-first word) and it doesn't look like a kubectl command, skip
	// suggestions — shell autocompletion is too complex to handle inline.
	if !hasKubectlPrefix && len(words) > 0 {
		firstWord := strings.ToLower(words[0])
		isKubectl := false
		for _, sub := range kubectlSubcommands() {
			if firstWord == sub {
				isKubectl = true
				break
			}
		}
		// If we're mid-word on the very first word, we still want to offer
		// kubectl subcommand completions so the user can discover them.
		// But if the first completed word isn't a kubectl subcommand, bail out.
		if !isKubectl && (!midWord || len(words) > 1) {
			return nil
		}
	}

	// Determine position: what word index are we completing?
	pos := len(words)
	if midWord && pos > 0 {
		pos-- // We're still typing the current word, so position is the last word's index.
	}

	var prefix string
	if midWord && len(words) > 0 {
		prefix = strings.ToLower(words[len(words)-1])
	}

	// --- Flag-aware completion ---
	// Check if the previous word is a flag that expects a value.
	if len(words) >= 1 {
		prevWord := ""
		if midWord && len(words) >= 2 {
			// Currently typing a word; the flag is the word before it.
			prevWord = words[len(words)-2]
		} else if !midWord {
			// Cursor is after a space; the flag is the last completed word.
			prevWord = words[len(words)-1]
		}

		prevWordLower := strings.ToLower(prevWord)
		switch prevWordLower {
		case "-n", "--namespace":
			return m.filterSuggestions(m.cachedNamespaces, prefix)
		case "-o", "--output":
			return m.filterSuggestions(outputFormatSuggestions(), prefix)
		}
	}

	// If the current word starts with "-", suggest common kubectl flags.
	if midWord && strings.HasPrefix(prefix, "-") {
		return m.filterSuggestions(kubectlFlagSuggestions(), prefix)
	}

	var candidates []string
	switch pos {
	case 0:
		// First word: kubectl subcommands + built-in commands.
		candidates = kubectlSubcommands()
	case 1:
		// Second word: depends on the subcommand.
		subCmd := ""
		if len(words) > 0 {
			subCmd = strings.ToLower(words[0])
		}
		switch subCmd {
		case "get", "describe", "delete", "edit", "patch", "label", "annotate", "scale", "autoscale":
			candidates = m.resourceTypeSuggestions()
		case "logs", "exec", "port-forward", "attach", "cp":
			// These operate on pods - suggest resource names if we have pods loaded.
			candidates = m.resourceNameSuggestions()
		case "apply", "create":
			candidates = []string{"-f", "-k", "--dry-run=client", "--dry-run=server"}
		case "rollout":
			candidates = []string{"status", "history", "undo", "restart", "pause", "resume"}
		case "config":
			candidates = []string{"view", "use-context", "get-contexts", "current-context", "set-context"}
		case "top":
			candidates = []string{"pods", "nodes"}
		case "cordon", "uncordon", "drain", "taint":
			candidates = m.resourceNameSuggestions()
		default:
			candidates = m.resourceTypeSuggestions()
		}
	default:
		// Third+ word: suggest resource names from the current view context.
		subCmd := ""
		if len(words) > 0 {
			subCmd = strings.ToLower(words[0])
		}
		switch subCmd {
		case "rollout":
			if pos == 2 {
				// After "rollout status/restart/etc", suggest resource types.
				candidates = m.resourceTypeSuggestions()
			} else {
				candidates = m.resourceNameSuggestions()
			}
		default:
			candidates = m.resourceNameSuggestions()
		}
	}

	if prefix == "" {
		// Limit visible suggestions when there's no prefix filter.
		const maxVisible = 8
		if len(candidates) > maxVisible {
			candidates = candidates[:maxVisible]
		}
		return candidates
	}

	// Filter candidates by prefix.
	var filtered []string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), prefix) && strings.ToLower(c) != prefix {
			filtered = append(filtered, c)
		}
	}

	const maxFiltered = 8
	if len(filtered) > maxFiltered {
		filtered = filtered[:maxFiltered]
	}

	return filtered
}

// kubectlSubcommands returns common kubectl subcommands for autocompletion.
func kubectlSubcommands() []string {
	return []string{
		"get", "describe", "logs", "exec", "delete", "apply", "create",
		"edit", "patch", "scale", "rollout", "top", "label", "annotate",
		"port-forward", "cp", "cordon", "uncordon", "drain", "taint",
		"config", "auth", "api-resources", "explain", "diff",
	}
}

// resourceTypeSuggestions returns resource type names available for completion.
// It combines the built-in types with any CRDs that were discovered.
func (m Model) resourceTypeSuggestions() []string {
	seen := make(map[string]bool)
	var result []string

	for _, cat := range model.TopLevelResourceTypes() {
		for _, t := range cat.Types {
			name := strings.ToLower(t.Resource)
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	// Also include CRD resource types from the left column items if available.
	for _, item := range m.leftItems {
		if item.Extra != "" && item.Extra != "__overview__" && item.Extra != "__monitoring__" && item.Kind != "__port_forwards__" {
			name := strings.ToLower(item.Name)
			if !seen[name] {
				seen[name] = true
				result = append(result, name)
			}
		}
	}

	return result
}

// resourceNameSuggestions returns resource names from the current middle column,
// which represents the resources currently visible to the user.
func (m Model) resourceNameSuggestions() []string {
	var names []string
	seen := make(map[string]bool)
	for _, item := range m.middleItems {
		if item.Name != "" && !seen[item.Name] {
			seen[item.Name] = true
			names = append(names, item.Name)
		}
	}
	return names
}

// kubectlFlagSuggestions returns common kubectl flags for autocompletion.
func kubectlFlagSuggestions() []string {
	return []string{
		"-n", "--namespace",
		"-o", "--output",
		"-l", "--selector",
		"-A", "--all-namespaces",
		"--sort-by",
		"--field-selector",
		"--show-labels",
		"-w", "--watch",
		"--no-headers",
		"-f", "--filename",
		"--dry-run=client", "--dry-run=server",
	}
}

// outputFormatSuggestions returns kubectl output format values.
func outputFormatSuggestions() []string {
	return []string{
		"json", "yaml", "wide", "name",
		"jsonpath=", "custom-columns=",
	}
}

// filterSuggestions filters candidates by prefix and returns a limited result set.
// This is used by flag-aware completion paths that bypass the main switch logic.
func (m Model) filterSuggestions(candidates []string, prefix string) []string {
	const maxResults = 8
	if prefix == "" {
		if len(candidates) > maxResults {
			candidates = candidates[:maxResults]
		}
		return candidates
	}
	var filtered []string
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToLower(c), prefix) && strings.ToLower(c) != prefix {
			filtered = append(filtered, c)
		}
	}
	if len(filtered) > maxResults {
		filtered = filtered[:maxResults]
	}
	return filtered
}
