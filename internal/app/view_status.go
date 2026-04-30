package app

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// broadModeSuffix names the extra match dimension Tab opens at the
// current level. At LevelResourceTypes Tab adds category-bar matches
// ("+groups"); everywhere else it scans column values ("+columns").
// Returned with parentheses so the caller can drop it next to the
// "filter"/"search" label without ad-hoc spacing logic.
func (m Model) broadModeSuffix() string {
	if m.nav.Level == model.LevelResourceTypes {
		return "(+groups)"
	}
	return "(+columns)"
}

// leftColumnHeader returns the header label for the left (parent) column.
func (m Model) leftColumnHeader() string {
	switch m.nav.Level {
	case model.LevelClusters:
		return "" // no parent at top level
	case model.LevelResourceTypes:
		return "KUBECONFIG"
	case model.LevelResources:
		return "RESOURCE TYPE"
	case model.LevelOwned:
		return strings.ToUpper(m.nav.ResourceType.DisplayName)
	case model.LevelContainers:
		return strings.ToUpper(m.nav.ResourceType.DisplayName)
	default:
		return ""
	}
}

// middleColumnHeader returns the header label for the middle column.
func (m Model) middleColumnHeader() string {
	switch m.nav.Level {
	case model.LevelClusters:
		return "KUBECONFIG"
	case model.LevelResourceTypes:
		return "RESOURCE TYPE"
	case model.LevelResources:
		return strings.ToUpper(m.nav.ResourceType.Kind)
	case model.LevelOwned:
		return strings.ToUpper(m.ownedItemKindLabel())
	case model.LevelContainers:
		return "CONTAINER"
	default:
		return ""
	}
}

// breadcrumb builds the "lfk > context > Type > Name > Owned" path rendered
// in the title bar. It mirrors labelForNav (see tabs.go) — both must use
// model.DisplayNameFor for the resource type because API-discovery-produced
// ResourceTypeEntry values do NOT populate DisplayName themselves. Reading
// nav.ResourceType.DisplayName directly silently drops the type for almost
// every real-world resource, leaving the title bar showing only the context.
func (m Model) breadcrumb() string {
	parts := []string{"lfk"}
	if m.nav.Context != "" {
		parts = append(parts, m.nav.Context)
	}
	if name := model.DisplayNameFor(m.nav.ResourceType); name != "" {
		parts = append(parts, name)
	}
	if m.nav.ResourceName != "" {
		parts = append(parts, m.nav.ResourceName)
	}
	// navigateChildResource sets both ResourceName and OwnedName to the same
	// value when entering a Pod (so the containers view knows its parent).
	// Skip the duplicate so the breadcrumb reads "lfk > ctx > Pods > my-pod"
	// instead of "lfk > ctx > Pods > my-pod > my-pod".
	if m.nav.OwnedName != "" && m.nav.OwnedName != m.nav.ResourceName {
		parts = append(parts, m.nav.OwnedName)
	}
	return strings.Join(parts, " > ")
}

// renderStatusHint paints m.statusMessage in the status-bar style at full
// width, suitable as a drop-in replacement for the bottom hint line of any
// fullscreen viewer. The caller must check hasStatusMessage() first.
func (m Model) renderStatusHint() string {
	innerWidth := max(m.width-2, 10)
	msg := m.sanitizeMessage(m.statusMessage)
	style := ui.StatusMessageOkStyle
	if m.statusMessageErr {
		style = ui.StatusMessageErrStyle
	}
	styled := ui.Truncate(style.Render(msg), innerWidth)
	return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(styled)
}

func (m Model) statusBar() string {
	// StatusBarBgStyle has Padding(0, 1) which adds 2 chars of horizontal padding.
	// Use MaxWidth on the content to prevent overflow.
	innerWidth := max(m.width-2, 10)

	// Show command bar when active.
	if m.commandBarActive {
		var prompt string
		if m.commandBarPreview != "" {
			// Ghost preview mode: show typed text + ghost completion (dimmed) + cursor at end.
			typed := m.commandBarInput.Value
			lastSpace := strings.LastIndex(typed, " ")
			partial := typed
			if lastSpace >= 0 {
				partial = typed[lastSpace+1:]
			}
			ghost := m.commandBarPreview
			lp := strings.ToLower(partial)
			lg := strings.ToLower(ghost)
			if strings.HasPrefix(lg, lp) {
				ghost = ghost[len(partial):]
			}
			prompt = ui.HelpKeyStyle.Render(":") +
				typed +
				ui.BarDimStyle.Render(ghost) +
				ui.BarDimStyle.Render("\u2588")
		} else {
			// Normal mode: overlay the cursor on the character at its position
			// (reverse-video) instead of inserting a block between characters.
			// Inserting would visually push the text after the cursor to the
			// right by one column and create the impression of a space.
			prompt = ui.HelpKeyStyle.Render(":") + renderInputWithCursor(m.commandBarInput.Value, m.commandBarInput.Cursor)
		}
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}

	// Show filter/search input in status bar when active. The
	// broad-mode suffix names what Tab actually adds at this level —
	// "+groups" at LevelResourceTypes (category bars), "+columns"
	// elsewhere (annotations, labels, CRD printer columns, custom
	// columns) — so the user knows what they just opted into.
	if m.filterActive {
		filterModeInd := ui.SearchModeIndicator(m.filterInput.Value)
		label := "filter"
		if m.filterBroadMode {
			label = "filter " + m.broadModeSuffix()
		}
		prompt := ui.HelpKeyStyle.Render(label) + ui.BarDimStyle.Render(": ") + ui.BarDimStyle.Render(filterModeInd) + renderInputWithCursor(m.filterInput.Value, m.filterInput.Cursor)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	if m.searchActive {
		searchModeInd := ui.SearchModeIndicator(m.searchInput.Value)
		label := "search"
		if m.searchBroadMode {
			label = "search " + m.broadModeSuffix()
		}
		prompt := ui.HelpKeyStyle.Render(label) + ui.BarDimStyle.Render(": ") + ui.BarDimStyle.Render(searchModeInd) + renderInputWithCursor(m.searchInput.Value, m.searchInput.Cursor)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).Render(prompt)
	}
	// When a status message is active, show it exclusively (hide key hints).
	if m.hasStatusMessage() {
		return m.renderStatusHint()
	}

	// When an overlay is active, show overlay-specific hints instead of
	// explorer hints. During a bulk action — the overlay was opened with
	// multiple items selected, e.g. bulk delete/drain confirms, the
	// confirm-type force-delete prompt, batch label editor — also pin
	// the selection-count badge to the right edge so the user keeps
	// "how many am I about to affect?" in view while reading the
	// confirm. Other overlays (theme picker, namespace selector, etc.)
	// do not carry that context, so the badge stays scoped to bulkMode.
	if hint := m.overlayHintBar(); hint != "" {
		var content string
		if m.bulkMode && len(m.bulkItems) > 0 {
			right := ui.SelectionCountStyle.Render(fmt.Sprintf(" %d selected ", len(m.bulkItems)))
			content = ui.JoinStatusBar(hint, right, innerWidth)
		} else {
			content = ui.Truncate(hint, innerWidth)
		}
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(content)
	}

	// When the help screen is active, show help-specific hints.
	if m.mode == modeHelp {
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(m.helpHintBar())
	}

	// When the error log overlay is active, show error log hints.
	if m.overlayErrorLog {
		var entries []ui.HintEntry
		switch m.errorLogVisualMode {
		case 'v':
			entries = []ui.HintEntry{
				{Key: "h/l", Desc: "column"},
				{Key: "j/k", Desc: "extend"},
				{Key: "0/$", Desc: "start/end"},
				{Key: "y", Desc: "copy"},
				{Key: "esc", Desc: "cancel"},
			}
		case 'V':
			entries = []ui.HintEntry{
				{Key: "j/k", Desc: "extend"},
				{Key: "y", Desc: "copy"},
				{Key: "esc", Desc: "cancel"},
			}
		default:
			debugHint := "show debug"
			if m.showDebugLogs {
				debugHint = "hide debug"
			}
			fsHint := "fullscreen"
			if m.errorLogFullscreen {
				fsHint = "overlay"
			}
			entries = []ui.HintEntry{
				{Key: "j/k", Desc: "scroll"},
				{Key: "V", Desc: "select"},
				{Key: "y", Desc: "copy all"},
				{Key: "f", Desc: fsHint},
				{Key: "d", Desc: debugHint},
				{Key: "esc", Desc: "close"},
			}
		}
		hint := m.renderHints(entries)
		return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(hint)
	}

	// Layout: the informational chip group (sort, counter / selected
	// count, filter preset, NYAN) anchors the FAR RIGHT; the keymap
	// hints fill the remaining space on the left. JoinStatusBar treats
	// the right side as priority — on overflow the keymap is the part
	// that yields. Overlay hint bars use a separate code path above and
	// are unaffected by this layout.
	right := m.explorerStatusChips()

	// Pre-fit the keymap to the left budget so JoinStatusBar never has to
	// hard-cut a hint mid-description. FormatHintPartsFit drops any entry
	// that wouldn't fit whole and continues scanning later entries — so the
	// reader sees only complete `key: desc` units, and the gap between the
	// keymap and the chip group is whitespace, never a stray `~`. Reserve
	// 2 columns so the gap between keymap and chips reads as a clear gutter
	// (the elastic-spacer minimum is 1, but a 2-col gutter is more comfortable).
	leftBudget := max(innerWidth-lipgloss.Width(right)-2, 0)
	left := ui.FormatHintPartsFit(m.explorerHintEntries(), leftBudget)

	content := ui.JoinStatusBar(left, right, innerWidth)
	return ui.StatusBarBgStyle.Width(m.width).MaxWidth(m.width).MaxHeight(1).Render(content)
}

// explorerStatusChips composes the right-anchored chip group for the
// explorer status bar: sort indicator, cursor counter or selection
// badge, active filter preset, and NYAN flag. Reading order is
// left-to-right; the chip group as a whole is right-aligned by
// JoinStatusBar in statusBar(). Returns the joined string ready to
// pass as the `right` argument to JoinStatusBar.
func (m Model) explorerStatusChips() string {
	visible := m.visibleMiddleItems()
	total := len(m.middleItems)
	cur := m.cursor() + 1

	var parts []string

	if m.sortApplies() {
		parts = append(parts, ui.BarDimStyle.Render("sort:"+m.sortModeName()))
	}
	// Counter ↔ selection swap. By default we surface the cursor position
	// inside the visible item list ("[1/47]"); the moment the user marks
	// at least one item we replace that chip with the selection-count
	// badge ("3 selected"). Showing both was redundant — the user knows
	// they're in bulk mode the second they kick off a selection — and the
	// stacked chips made the bar grow noticeably wider, which in turn
	// pushed the keymap into truncation territory and was perceived as
	// "the hint bar gets trimmed when I select items".
	if m.hasSelection() {
		parts = append(parts, ui.SelectionCountStyle.Render(fmt.Sprintf(" %d selected ", len(m.selectedItems))))
	} else if m.filterText != "" {
		parts = append(parts, ui.BarDimStyle.Render(fmt.Sprintf("[%d/%d filtered: %d/%d]", cur, len(visible), len(visible), total)))
	} else {
		parts = append(parts, ui.BarDimStyle.Render(fmt.Sprintf("[%d/%d]", cur, total)))
	}

	// Filter preset and NYAN are decorations rather than primary signals,
	// so they sit at the far-right tail — first to be visually pushed
	// against the right edge, last in the chip group's reading order.
	if m.activeFilterPreset != nil {
		parts = append(parts, ui.HelpKeyStyle.Render("[filter: "+m.activeFilterPreset.Name+"]"))
	}
	if m.nyanMode {
		parts = append(parts, ui.HelpKeyStyle.Render("[NYAN]"))
	}

	return strings.Join(parts, "  ")
}

// explorerHintEntries builds the bottom hint bar for explorer views (cluster
// picker, resource-type browser, resource list). Extracted from statusBar to
// keep that function under the gocyclo budget. Dashboard views use a reduced
// set; standard explorer views use the full set with conditional hides for
// keys that are no-ops at the current level.
func (m Model) explorerHintEntries() []ui.HintEntry {
	kb := ui.ActiveKeybindings
	sel := m.selectedMiddleItem()
	isDashboard := sel != nil && m.nav.Level == model.LevelResourceTypes &&
		(sel.Extra == "__overview__" || sel.Extra == "__monitoring__")
	if isDashboard {
		return []ui.HintEntry{
			{Key: kb.Down + "/" + kb.Up, Desc: "move"},
			{Key: kb.PageDown + "/" + kb.PageUp, Desc: "scroll"},
			{Key: kb.NamespaceSelector, Desc: "namespace"},
			{Key: kb.NewTab, Desc: "new tab"},
			{Key: kb.Help, Desc: "help"},
			{Key: "q", Desc: "quit"},
		}
	}

	hintEntries := []ui.HintEntry{
		{Key: kb.Left + "/" + kb.Right, Desc: "navigate"},
		{Key: kb.Down + "/" + kb.Up, Desc: "move"},
		{Key: kb.Enter, Desc: "view"},
		{Key: kb.NamespaceSelector, Desc: "namespace"},
		{Key: kb.AllNamespaces, Desc: "all-ns"},
	}
	// At the cluster picker and resource-type browser, both the action
	// menu and column sort are no-ops: selectedResourceKind() returns
	// "" so openActionMenu() bails out, and sortMiddleItems() early-
	// returns so </> doesn't reorder anything. Hide both hints there
	// to avoid advertising dead keys.
	hasResourceContext := m.nav.Level != model.LevelClusters && m.nav.Level != model.LevelResourceTypes
	if hasResourceContext {
		hintEntries = append(hintEntries, ui.HintEntry{Key: kb.ActionMenu, Desc: "actions"})
	}
	// Advertise the read-only toggle on every level so users can
	// discover it without reading docs. At the cluster picker it
	// flips a row marker; inside a context it locks/unlocks the
	// active tab. Hidden inside a context when --read-only is set,
	// since the gate rejects the toggle.
	if m.nav.Level == model.LevelClusters || !m.cliReadOnly {
		hintEntries = append(hintEntries, ui.HintEntry{Key: kb.ReadOnlyToggle, Desc: "toggle RO"})
	}
	// "create" runs `kubectl apply` from a template. Hide it in
	// read-only mode since it would be blocked anyway.
	if !m.readOnly {
		hintEntries = append(hintEntries, ui.HintEntry{Key: kb.CreateTemplate, Desc: "create"})
	}
	if hasResourceContext {
		hintEntries = append(hintEntries, ui.HintEntry{Key: kb.SortNext + "/" + kb.SortPrev, Desc: "sort"})
	}
	hintEntries = append(hintEntries,
		ui.HintEntry{Key: kb.Filter, Desc: "filter"},
		ui.HintEntry{Key: kb.SetMark + "/" + kb.OpenMarks, Desc: "marks"},
		ui.HintEntry{Key: kb.Help, Desc: "help"},
		ui.HintEntry{Key: "q", Desc: "quit"},
	)
	return m.appendEventsHintEntries(hintEntries)
}

// appendEventsHintEntries injects Events-view toggle hints (warnings-only,
// grouping) just before the trailing "quit" entry. Returns the input slice
// unchanged when the current view isn't the Events resource list.
//
// Callers must keep "quit" as the final entry so it always renders on the
// right edge of the hint bar. If that invariant changes, this helper's
// splice (entries[:len(entries)-1]) will misplace the toggles.
func (m Model) appendEventsHintEntries(entries []ui.HintEntry) []ui.HintEntry {
	if m.nav.Level != model.LevelResources || m.nav.ResourceType.Kind != "Event" {
		return entries
	}
	kb := ui.ActiveKeybindings
	warnDesc := "warnings only"
	if m.warningEventsOnly {
		warnDesc = "all events"
	}
	groupDesc := "group"
	if m.eventGrouping {
		groupDesc = "ungroup"
	}
	extras := []ui.HintEntry{
		{Key: kb.SaveResource, Desc: warnDesc},
		{Key: kb.ExpandCollapse, Desc: groupDesc},
	}
	if len(entries) == 0 {
		return extras
	}
	// Insert before the trailing "quit" entry so the Events toggles sit next
	// to the other contextual actions.
	out := make([]ui.HintEntry, 0, len(entries)+len(extras))
	out = append(out, entries[:len(entries)-1]...)
	out = append(out, extras...)
	out = append(out, entries[len(entries)-1])
	return out
}

// renderInputWithCursor renders a text input value with a reverse-video
// cursor overlaid on the character at the given byte position. When the
// cursor is past the end of the value, a highlighted space is appended.
//
// Unlike the previous "insert block between characters" approach, this
// overlays the cursor on the character it points at, so moving left or
// right does not push the surrounding text around and create the
// appearance of an inserted space.
func renderInputWithCursor(value string, cursor int) string {
	if cursor < 0 {
		cursor = 0
	}
	if cursor >= len(value) {
		return value + ui.CursorBlockStyle.Render(" ")
	}
	return value[:cursor] + ui.CursorBlockStyle.Render(string(value[cursor])) + value[cursor+1:]
}

// helpHintBar renders the status-bar hint line for the help screen,
// switching shape based on which input/applied state is active.
// Extracted from statusBar to keep that function under the gocyclo
// budget; the help screen has five distinct prompt shapes.
func (m Model) helpHintBar() string {
	switch {
	case m.helpSearchActive:
		// Live search input — show the typed query plus a hint that
		// ctrl+n/p navigate matches in real time.
		return ui.HelpKeyStyle.Render("search") + ui.BarDimStyle.Render(": ") + m.helpSearchInput.View() +
			"  " + ui.HelpKeyStyle.Render("^n/^p") + ui.BarDimStyle.Render(" next/prev") +
			"  " + ui.HelpKeyStyle.Render("Enter") + ui.BarDimStyle.Render(" apply") +
			"  " + ui.HelpKeyStyle.Render("Esc") + ui.BarDimStyle.Render(" cancel")
	case m.helpFilterActive:
		return ui.HelpKeyStyle.Render("filter") + ui.BarDimStyle.Render(": ") + m.helpSearchInput.View() +
			"  " + ui.HelpKeyStyle.Render("Enter") + ui.BarDimStyle.Render(" apply") +
			"  " + ui.HelpKeyStyle.Render("Esc") + ui.BarDimStyle.Render(" cancel")
	case m.helpSearchQuery != "":
		// Search applied — n/N navigates persisted matches.
		return ui.BarDimStyle.Render("search: ") +
			ui.HelpKeyStyle.Render(m.helpSearchQuery) + "  " +
			ui.HelpKeyStyle.Render("n/N") + ui.BarDimStyle.Render(" next/prev") + "  " +
			ui.HelpKeyStyle.Render("/") + ui.BarDimStyle.Render(" edit") + "  " +
			ui.HelpKeyStyle.Render("Esc") + ui.BarDimStyle.Render(" clear")
	case m.helpFilter.Value != "":
		return ui.BarDimStyle.Render("filter: ") +
			ui.HelpKeyStyle.Render(m.helpFilter.Value) + "  " +
			ui.HelpKeyStyle.Render("f") + ui.BarDimStyle.Render(" edit") + "  " +
			ui.HelpKeyStyle.Render("Esc") + ui.BarDimStyle.Render(" close")
	default:
		return m.renderHints([]ui.HintEntry{
			{Key: "j/k", Desc: "scroll"},
			{Key: "^d/^u", Desc: "half-page"},
			{Key: "/", Desc: "search"},
			{Key: "f", Desc: "filter"},
			{Key: "Esc/?/q", Desc: "close"},
		})
	}
}
