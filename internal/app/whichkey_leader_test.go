package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestWhichKeyLeader_ConfiguredDelayHidesUntilTick keeps the delay knob
// covered even though it now defaults to 0: a user who sets
// which_key_leader_delay_ms must still get a hidden panel until the tick.
func TestWhichKeyLeader_ConfiguredDelayHidesUntilTick(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300
	m := whichKeyTestModel()

	out, _ := m.handleExplorerKey(leaderKey())
	got := out.(Model)
	if !got.whichKey.armed {
		t.Fatal("the leader key must arm the which-key panel")
	}
	if got.whichKey.shown {
		t.Fatal("panel must stay hidden until the delay elapses")
	}
}

func TestWhichKeyLeader_ZeroDelayShowsImmediately(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	out, _ := m.handleExplorerKey(leaderKey())
	if !out.(Model).whichKey.shown {
		t.Fatal("a zero delay must reveal the panel immediately")
	}
}

func TestWhichKeyLeaderTick_IgnoresStaleGeneration(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.whichKey = whichKeyState{armed: true, seq: 7}
	out, _, _ := m.updateEasterEggMsg(whichKeyLeaderTickMsg{seq: 6})
	if out.(Model).whichKey.shown {
		t.Fatal("a tick from a superseded arming must not reveal the panel")
	}
	out, _, _ = m.updateEasterEggMsg(whichKeyLeaderTickMsg{seq: 7})
	if !out.(Model).whichKey.shown {
		t.Fatal("the current generation's tick must reveal the panel")
	}
}

func TestWhichKeyLeaderTick_IgnoredWhenDisarmed(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.whichKey = whichKeyState{armed: false, seq: 3}
	out, _, _ := m.updateEasterEggMsg(whichKeyLeaderTickMsg{seq: 3})
	if out.(Model).whichKey.shown {
		t.Fatal("a tick must not reveal the panel after disarm")
	}
}

func TestWhichKeyLeader_EscDisarmsAndIsConsumed(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	armed, _ := m.handleExplorerKey(leaderKey())
	before := len(armed.(Model).selectedItems)

	out, _ := armed.(Model).handleExplorerKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	got := out.(Model)
	if got.whichKey.armed || got.whichKey.shown {
		t.Fatal("esc must disarm the leader")
	}
	if len(got.selectedItems) != before {
		t.Fatal("esc must be consumed by the leader, not clear the selection")
	}
}

func TestWhichKeyLeader_OtherKeyDisarmsAndStillActs(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	ui.ActiveKeybindings = kb
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.setMiddleItems([]model.Item{
		{Name: "p1", Kind: "Pod", Namespace: "default"},
		{Name: "p2", Kind: "Pod", Namespace: "default"},
	})
	// Start on the second row so the "up" key below has somewhere to go: the
	// leader no longer moves the cursor itself.
	m.setCursor(1)

	armedOut, _ := m.handleExplorerKey(leaderKey())
	armed := armedOut.(Model)
	if !armed.whichKey.armed {
		t.Fatal("precondition: the leader key must arm")
	}
	cursorBefore := armed.cursor()
	out, _ := armed.handleExplorerKey(tea.KeyPressMsg{Code: rune(kb.Up[0]), Text: kb.Up})
	got := out.(Model)
	if got.whichKey.armed {
		t.Fatal("an unlisted key must disarm the leader")
	}
	if got.cursor() == cursorBefore {
		t.Fatal("an unlisted key must still perform its normal action")
	}
}

func TestWhichKeyLeader_HiddenWhenDisabled(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = false
	m := whichKeyTestModel()
	m.whichKey = whichKeyState{armed: true, shown: true}
	bg := strings.Repeat("\n", m.height)
	if got := m.renderWhichKeyLeader(bg); got != bg {
		t.Fatal("panel must not render when which_key_enabled is false")
	}
}

// TestWhichKeyLeaderCells_AreOneSortedListAcrossGroups replaces the old
// "groups come out in whichKeyGroupOrder" assertion. The panel is a single flat
// list now, so the property that matters is that the whole list is in
// which-key order end to end, INCLUDING the group clustering pass —
// sortWhichKeyCells is the single source of truth for the list's order, not
// just its within-group tail.
//
// This used to assert the opposite — that entries from different catalog
// groups interleave — back when the sort was pure-key with no group pass. That
// was deliberately the point being tested before: a flat list without even a
// group-based clustering. Grouping was added specifically so each group's
// color would form a contiguous, readable run instead of scattering across the
// grid, which requires entries to NOT interleave across groups; see
// TestWhichKeyLeaderCells_GroupsFormContiguousRuns for that property.
func TestWhichKeyLeaderCells_AreOneSortedListAcrossGroups(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	cells := m.whichKeyLeaderCells()
	if len(cells) == 0 {
		t.Fatal("a resource row must yield at least one entry")
	}
	if got, want := len(cells), len(m.availableWhichKeyActions()); got != want {
		t.Fatalf("the flat list holds %d entries, want every available action (%d)", got, want)
	}
	sorted := slices.Clone(cells)
	sortWhichKeyCells(sorted, true)
	if !slices.Equal(cells, sorted) {
		t.Fatalf("the flat list is not in which-key order:\n got %v\nwant %v", cells, sorted)
	}
	groups := map[whichKeyGroup]bool{}
	for _, a := range m.availableWhichKeyActions() {
		groups[a.Group] = true
	}
	if len(groups) < 2 {
		t.Fatalf("precondition: the fixture must span several catalog groups, got %d", len(groups))
	}
}

// TestWhichKeyLeaderCells_GroupsFormContiguousRuns is the end-to-end version
// of TestSortWhichKeyCells_ClustersByGroupThenKey, run against the real
// catalog rather than a synthetic fixture: with the full set of available
// actions, each group's description color must land as one unbroken run, in
// whichKeyGroupOrder order, so the panel's color reads as structure instead
// of noise.
func TestWhichKeyLeaderCells_GroupsFormContiguousRuns(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	cells := m.whichKeyLeaderCells()

	order := whichKeyGroupOrder()
	rankOf := map[whichKeyGroup]int{}
	for i, g := range order {
		rankOf[g] = i
	}

	seenGroups := []whichKeyGroup{}
	for i, c := range cells {
		if i == 0 || cells[i-1].group != c.group {
			if len(seenGroups) > 0 && seenGroups[len(seenGroups)-1] == c.group {
				t.Fatalf("group %q appears in two separate runs, not one contiguous block", c.group)
			}
			seenGroups = append(seenGroups, c.group)
		}
	}
	if len(seenGroups) < 2 {
		t.Fatalf("precondition: the fixture must span several catalog groups, got %v", seenGroups)
	}
	for i := 1; i < len(seenGroups); i++ {
		if rankOf[seenGroups[i-1]] >= rankOf[seenGroups[i]] {
			t.Fatalf("group runs are not in whichKeyGroupOrder order: %v", seenGroups)
		}
	}
}

func TestWhichKeyLeader_DoesNotArmWhileFiltering(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.filterActive = true
	out, _ := m.Update(leaderKey())
	if out.(Model).whichKey.armed {
		t.Fatal("typing the leader key into the filter must not arm the leader")
	}
}

// TestWhichKeyLeader_RepeatPressTogglesTheGrouping pins the current USER
// DECISION for the repeat press. With the panel scrolling there is nothing for
// it to advance to; it used to close the panel, and now it toggles the entry
// order between grouped-by-category and pure key order, indefinitely, leaving
// the panel open. esc closes. The chosen order persists for the session.
func TestWhichKeyLeader_RepeatPressTogglesTheGrouping(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.armed || !m.whichKey.shown {
		t.Fatal("the first press must open the panel")
	}

	m, scrolled := m.scrollWhichKey(m.whichKeyLeaderCells(), true)
	if !scrolled || m.whichKey.scroll == 0 {
		t.Fatal("precondition: the full catalog must be scrollable at 80x24")
	}

	// USER DECISION: a repeat press no longer closes the panel — it toggles the
	// entry order and keeps it open, indefinitely. esc is the way out.
	grouped := m.whichKeyGrouped()
	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.armed || !m.whichKey.shown {
		t.Fatal("a repeat press must leave the panel open")
	}
	if m.whichKeyGrouped() == grouped {
		t.Fatal("a repeat press must flip the entry order")
	}
	if m.whichKey.scroll != 0 {
		t.Fatalf("the list reorders wholesale, so the offset must reset; got scroll=%d", m.whichKey.scroll)
	}

	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("a third press must still leave the panel open")
	}
	if m.whichKeyGrouped() != grouped {
		t.Fatal("a third press must flip the entry order back")
	}

	out, _ = m.handleExplorerKey(keyMsg("esc"))
	m = out.(Model)
	if m.whichKey.armed || m.whichKey.shown {
		t.Fatal("esc must close the panel")
	}
	// The mode is session state: reopening keeps whatever the last toggle left.
	m = m.toggleWhichKeyGrouping()
	want := m.whichKeyGrouped()
	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if m.whichKeyGrouped() != want {
		t.Fatal("reopening must keep the entry order the last toggle chose")
	}
}

// TestWhichKeyLeader_ScrollKeysMoveTheViewport: ctrl+d/ctrl+u scroll the panel
// by half a viewport and clamp at both ends.
func TestWhichKeyLeader_ScrollKeysMoveTheViewport(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	lay, ok := m.whichKeyLayoutFor(m.whichKeyLeaderCells())
	if !ok || lay.maxScroll == 0 {
		t.Fatalf("precondition: the full catalog must overflow at 80x24; maxScroll=%d", lay.maxScroll)
	}

	kb := ui.ActiveKeybindings
	out, _ = m.handleExplorerKey(keyMsg(kb.PageDown))
	m = out.(Model)
	if !m.whichKey.armed || !m.whichKey.shown {
		t.Fatal("a scroll key must not close the panel")
	}
	if want := min(max(lay.viewRows/2, 1), lay.maxScroll); m.whichKey.scroll != want {
		t.Fatalf("ctrl+d moved to %d, want a half page (%d)", m.whichKey.scroll, want)
	}

	for range 20 {
		out, _ = m.handleExplorerKey(keyMsg(kb.PageDown))
		m = out.(Model)
	}
	if m.whichKey.scroll != lay.maxScroll {
		t.Fatalf("scrolling past the end must clamp to %d, got %d", lay.maxScroll, m.whichKey.scroll)
	}

	for range 20 {
		out, _ = m.handleExplorerKey(keyMsg(kb.PageUp))
		m = out.(Model)
	}
	if m.whichKey.scroll != 0 {
		t.Fatalf("scrolling back past the top must clamp to 0, got %d", m.whichKey.scroll)
	}
	if !m.whichKey.armed {
		t.Fatal("scrolling must leave the panel open throughout")
	}
}

// TestWhichKeyLeader_ScrollKeysActNormallyWhenPanelHidden is the dispatch trap
// the scroll exemption creates: ctrl+d/ctrl+u are the explorer's own half-page
// cursor keys, so the exemption must apply ONLY while the panel is on screen.
// Covers both "never armed" and "armed but still inside the reveal delay".
func TestWhichKeyLeader_ScrollKeysActNormallyWhenPanelHidden(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	kb := ui.ActiveKeybindings

	rows := make([]model.Item, 40)
	for i := range rows {
		rows[i] = model.Item{Name: fmt.Sprintf("p%d", i), Kind: "Pod", Namespace: "default"}
	}

	t.Run("never armed", func(t *testing.T) {
		ui.ConfigWhichKeyLeaderDelayMs = 0
		m := whichKeyTestModel()
		m.width, m.height = 80, 24
		m.setMiddleItems(rows)
		m.setCursor(0)
		out, _ := m.handleExplorerKey(keyMsg(kb.PageDown))
		moved := out.(Model)
		if moved.cursor() == 0 {
			t.Fatal("ctrl+d must still page the list when no panel is shown")
		}
	})

	t.Run("armed but not yet shown", func(t *testing.T) {
		ui.ConfigWhichKeyLeaderDelayMs = 300
		m := whichKeyTestModel()
		m.width, m.height = 80, 24
		m.setMiddleItems(rows)
		m.setCursor(0)
		out, _ := m.handleExplorerKey(leaderKey())
		m = out.(Model)
		if !m.whichKey.armed || m.whichKey.shown {
			t.Fatalf("precondition: armed=%v shown=%v at a 300ms delay", m.whichKey.armed, m.whichKey.shown)
		}
		out, _ = m.handleExplorerKey(keyMsg(kb.PageDown))
		got := out.(Model)
		if got.whichKey.armed {
			t.Fatal("a key arriving before the reveal must still close the leader")
		}
		if got.cursor() == 0 {
			t.Fatal("ctrl+d must still page the list while the panel is invisible")
		}
	})
}

// TestWhichKeyLeader_ScrollResetsOnDisarmAndRearm: the offset lives on
// whichKeyState and must not survive a close/reopen.
func TestWhichKeyLeader_ScrollResetsOnDisarmAndRearm(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
	m = out.(Model)
	if m.whichKey.scroll == 0 {
		t.Fatal("precondition: ctrl+d should have scrolled")
	}

	out, _ = m.handleExplorerKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = out.(Model)
	if m.whichKey.scroll != 0 {
		t.Fatalf("disarm (esc) must reset the scroll offset, got %d", m.whichKey.scroll)
	}

	out, _ = m.handleExplorerKey(leaderKey())
	if got := out.(Model).whichKey.scroll; got != 0 {
		t.Fatalf("re-arming must start at the top, got %d", got)
	}
}

// TestSpace_MultiSelectNeverArmsTheLeader replaces the old "space j space j
// must not flash the panel" scenario: the leader has moved off space, so the
// multi-select burst can no longer arm anything at all — a stronger property
// than the delay-based avoidance it supersedes. Runs at a nonzero delay so a
// stray arming would still be caught before its reveal tick.
func TestSpace_MultiSelectNeverArmsTheLeader(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	ui.ActiveKeybindings = kb
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300
	m := whichKeyTestModel()
	m.setMiddleItems([]model.Item{
		{Name: "p1", Kind: "Pod", Namespace: "default"},
		{Name: "p2", Kind: "Pod", Namespace: "default"},
		{Name: "p3", Kind: "Pod", Namespace: "default"},
	})
	m.setCursor(0)

	downKey := tea.KeyPressMsg{Code: rune(kb.Down[0]), Text: kb.Down}
	for range 2 {
		out, _ := m.handleExplorerKey(spaceKey())
		m = out.(Model)
		if m.whichKey.armed || m.whichKey.shown {
			t.Fatal("space must never arm the which-key leader")
		}
		out, _ = m.handleExplorerKey(downKey)
		m = out.(Model)
	}
	if len(m.selectedItems) != 2 {
		t.Fatalf("space j space j must multi-select exactly two rows; selected=%d", len(m.selectedItems))
	}
}

// TestWhichKeyLeader_OpensOnTheFirstSortedEntriesAt80x24 replaces the old
// "opens on the Actions group" assertion, which pinned a header the panel no
// longer draws. With one flat list the equivalent property is that an
// unscrolled panel starts at the top of that list — the first entry of every
// column's first row is on screen.
func TestWhichKeyLeader_OpensOnTheFirstSortedEntriesAt80x24(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 80, 24
	m.whichKey.armed = true
	m.whichKey.shown = true

	cells := m.whichKeyLeaderCells()
	if len(cells) == 0 {
		t.Fatal("the panel must not be empty at 80x24")
	}
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		t.Fatal("the panel must lay out at 80x24")
	}
	if lay.maxScroll == 0 {
		t.Fatal("expected the full catalog to overflow at 80x24")
	}
	out := stripANSI(m.renderWhichKeyLeader(strings.Repeat("\n", m.height)))
	for b := range lay.grid.boxN {
		first := cells[b*lay.grid.rowN]
		want := first.keyText() + " " + ui.Truncate(first.desc, lay.grid.descW)
		if !strings.Contains(out, want) {
			t.Fatalf("the unscrolled panel must open on column %d's first entry %q:\n%s", b, want, out)
		}
	}
}

// TestWhichKeyLeaderPage_NeverEmptyBoxAtShortHeights is CRITICAL-2 from
// review round 1: at realistic short terminal heights (80x11, 80x12 — a
// routine tmux split-pane size), the panel rendered a box containing only
// "+N more (? for help)" and no real content — a triple footer reservation
// (leader pagination, renderWhichKeyPanel, and fitWhichKeyGroups each
// reserving their own row) starved the cell budget to zero, and the
// len(body)==0 empty-content guard couldn't fire because the footer line was
// appended before that check ran. The panel must now either show real
// entries or not render at all — never a box with nothing actionable in it.
func TestWhichKeyLeader_NeverEmptyBoxAtShortHeights(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	for _, h := range []int{11, 12} {
		m := whichKeyTestModel()
		m.width, m.height = 80, h
		m.whichKey.armed = true
		m.whichKey.shown = true
		bg := strings.Repeat("\n", m.height)
		out := stripANSI(m.renderWhichKeyLeader(bg))
		if out == bg {
			continue // nothing rendered at all — acceptable per spec
		}
		// Headers are gone, so "has real content" now means the first entry of
		// the flat list is actually drawn.
		cells := m.whichKeyLeaderCells()
		lay, ok := m.whichKeyLayoutFor(cells)
		if !ok {
			t.Errorf("h=%d: a rendered panel must lay out", h)
			continue
		}
		want := cells[0].keyText() + " " + ui.Truncate(cells[0].desc, lay.grid.descW)
		if !strings.Contains(out, want) {
			t.Errorf("CRITICAL-2: h=%d rendered a box with no real content (missing %q):\n%s", h, want, out)
		}
	}
}

// TestWhichKeyLeader_AllEntriesReachableViaScrolling is IMPORTANT-3 from review
// round 1, rewritten for scrolling: no entry may be unreachable. The old paging
// version scanned every page; this scans every scroll offset, and — because the
// scroll offsets are driven through the real ctrl+d key path rather than by
// poking the state — it also proves the scroll keys actually reach the end.
// Asserts on the rendered text, not the group data the renderer might still
// clip.
func TestWhichKeyLeader_AllEntriesReachableViaScrolling(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	for _, size := range [][2]int{{80, 14}, {80, 24}, {120, 40}} {
		m := whichKeyTestModel()
		m.width, m.height = size[0], size[1]

		want := map[string]bool{}
		for _, a := range m.availableWhichKeyActions() {
			want[a.Label] = true
		}

		out, _ := m.handleExplorerKey(leaderKey())
		m = out.(Model)
		cells := m.whichKeyLeaderCells()
		lay, ok := m.whichKeyLayoutFor(cells)
		if !ok {
			t.Fatalf("%dx%d: the panel must lay out", size[0], size[1])
		}
		if len(cells) != len(want) {
			t.Fatalf("%dx%d: the flat list holds %d entries, want every available action (%d)",
				size[0], size[1], len(cells), len(want))
		}

		bg := strings.Repeat("\n", m.height)
		var rendered strings.Builder
		rendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
		for step := 0; m.whichKey.scroll < lay.maxScroll; step++ {
			if step > lay.bodyRows {
				t.Fatalf("%dx%d: ctrl+d never reached the end (%d of %d)", size[0], size[1], m.whichKey.scroll, lay.maxScroll)
			}
			out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
			m = out.(Model)
			rendered.WriteString("\n")
			rendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
		}
		// A label wider than the description field is ellipsized, so match the
		// text the panel is actually supposed to draw rather than the raw label.
		for _, c := range cells {
			drawn := c.keyText() + " " + ui.Truncate(c.desc, lay.grid.descW)
			if !strings.Contains(rendered.String(), drawn) {
				t.Errorf("%dx%d: %q never appears at any scroll offset — unreachable", size[0], size[1], drawn)
			}
		}
	}
}

// TestWhichKeyLeader_HintBarGatesOnShownNotArmed is IMPORTANT-4 from review
// round 1: the hint bar used to switch to the leader's "space: more / esc:
// close" hints as soon as the leader armed, not once the panel actually
// appeared. With a configured delay that meant the whole status bar — the
// selected count, sort indicator, and position counter included — was replaced
// for the length of the delay, even though the panel was invisible throughout.
func TestWhichKeyLeader_HintBarGatesOnShownNotArmed(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if m.whichKey.shown {
		t.Fatal("precondition: fresh arm must not be shown yet at 300ms delay")
	}
	// "?: more" is gone with paging; the leader hints are now the scroll keys
	// (only while overflowing) and esc.
	leaderHint := ui.ActiveKeybindings.PageDown + "/" + ui.ActiveKeybindings.PageUp + ": scroll"
	if hint := stripANSI(m.statusBar()); strings.Contains(hint, leaderHint) {
		t.Fatalf("IMPORTANT-4: hint bar switched to leader hints before the panel is shown:\n%s", hint)
	}

	out, _, _ = m.updateEasterEggMsg(whichKeyLeaderTickMsg{seq: m.whichKey.seq})
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("precondition: the tick must reveal the panel")
	}
	if hint := stripANSI(m.statusBar()); !strings.Contains(hint, leaderHint) {
		t.Fatalf("hint bar must show leader hints once the panel is actually visible:\n%s", hint)
	}
}

// TestWhichKeyLeader_MouseToggleKeyDisarms is IMPORTANT-5 from review round
// 1: handleMouseToggleKey (and handleTabSwitchKey, handleModeKey) all run
// before handleExplorerKey in handleKey's dispatch chain, so the only disarm
// guard living inside handleExplorerKey never saw keys those handlers
// claimed first — ctrl+alt+y (the default mouse-capture toggle, itself an
// entry the panel's own Settings group advertises) left the leader armed and
// visually stuck open. Routes through handleKey (via mouseToggleKey, the
// shared fixture in update_mouse_toggle_test.go) so the real dispatch order
// is exercised, not just handleExplorerKey in isolation.
func TestWhichKeyLeader_MouseToggleKeyDisarms(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.mouseAvailable = true

	armedOut, _ := m.handleExplorerKey(leaderKey())
	armed := armedOut.(Model)
	if !armed.whichKey.armed {
		t.Fatal("precondition: space must arm the leader")
	}

	out, _ := armed.handleKey(mouseToggleKey)
	got := out.(Model)
	if got.whichKey.armed || got.whichKey.shown {
		t.Fatal("IMPORTANT-5: the mouse-capture toggle key must close the leader")
	}
}

// TestWhichKeyLeader_MouseInputDisarms is the other half of IMPORTANT-5:
// actual mouse events (clicks, scroll) never reach any key-based disarm
// guard at all, since they arrive as tea.MouseMsg, not tea.KeyPressMsg — the
// leader stayed armed and overlaid while the user scrolled or clicked.
func TestWhichKeyLeader_MouseInputDisarms(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()

	armedOut, _ := m.handleExplorerKey(leaderKey())
	armed := armedOut.(Model)
	if !armed.whichKey.armed {
		t.Fatal("precondition: space must arm the leader")
	}

	out, _ := armed.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	got := out.(Model)
	if got.whichKey.armed || got.whichKey.shown {
		t.Fatal("IMPORTANT-5: mouse input must close the leader")
	}
}

// TestWhichKeyLeader_ScrollHintOnlyWhenOverflowing replaces the page-indicator
// pair. which-key.nvim adds its "<c-d>/<c-u> scroll" help entry only when the
// content overflows (view.lua:447-449); advertising it on a panel that already
// fits promises movement that cannot happen, and omitting it on one that
// doesn't hides the only way to see the rest.
func TestWhichKeyLeader_ScrollHintOnlyWhenOverflowing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	kb := ui.DefaultKeybindings()
	scrollHint := kb.PageDown + "/" + kb.PageUp

	t.Run("overflowing", func(t *testing.T) {
		ui.ActiveKeybindings = kb
		m := whichKeyTestModel()
		m.width, m.height = 80, 14
		m.whichKey.armed = true
		m.whichKey.shown = true
		if lay, _ := m.whichKeyLayoutFor(m.whichKeyLeaderCells()); lay.maxScroll == 0 {
			t.Fatal("precondition: the full catalog must overflow at 80x14")
		}
		if bar := stripANSI(m.statusBar()); !strings.Contains(bar, scrollHint) {
			t.Fatalf("an overflowing panel must advertise the scroll keys; got %q", bar)
		}
	})

	t.Run("fits", func(t *testing.T) {
		// availableWhichKeyActions drops entries whose key the user cleared, so
		// clearing all but two leaves a catalog that fits any real terminal.
		small := ui.Keybindings{}
		small.Refresh = "R"
		small.CopyName = "y"
		small.PageDown, small.PageUp = kb.PageDown, kb.PageUp
		small.WhichKeyLeader = kb.WhichKeyLeader
		ui.ActiveKeybindings = small

		m := whichKeyTestModel()
		m.width, m.height = 120, 40
		m.whichKey.armed = true
		m.whichKey.shown = true
		if lay, _ := m.whichKeyLayoutFor(m.whichKeyLeaderCells()); lay.maxScroll != 0 {
			t.Fatal("precondition: a two-entry catalog must fit at 120x40")
		}
		if bar := stripANSI(m.statusBar()); strings.Contains(bar, scrollHint) {
			t.Fatalf("a panel that fits must not advertise the scroll keys; got %q", bar)
		}
		if bar := stripANSI(m.statusBar()); !strings.Contains(bar, "esc") {
			t.Fatalf("the close hint must always render while the panel is up; got %q", bar)
		}
	})
}

// TestWhichKeyLeader_ScrollKeyFallsThroughWhenNothingToScroll: at a size where
// the whole catalog is on screen there is nowhere to scroll to, so ctrl+d must
// NOT be swallowed. It used to be consumed regardless, which lost the key's
// normal half-page list scroll for no visible effect — and contradicted both
// the hint bar (which already omits the scroll hint in that state) and
// docs/keybindings.md ("any other key: close, and still run normally").
func TestWhichKeyLeader_ScrollKeyFallsThroughWhenNothingToScroll(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := whichKeyTestModel()
	m.width, m.height = 200, 40
	rows := make([]model.Item, 60)
	for i := range rows {
		rows[i] = model.Item{Name: fmt.Sprintf("p%02d", i), Kind: "Pod", Namespace: "default"}
	}
	m.setMiddleItems(rows)
	m.setCursor(0)

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	lay, ok := m.whichKeyLayoutFor(m.whichKeyLeaderCells())
	if !ok || lay.maxScroll != 0 {
		t.Fatalf("precondition: the panel must fit whole at 200x40 (ok=%v maxScroll=%d)", ok, lay.maxScroll)
	}
	for _, h := range m.whichKeyPopupHints(m.whichKeyLeaderCells(), true) {
		if h.Desc == "scroll" {
			t.Fatal("precondition: the hint bar must not advertise scrolling when nothing scrolls")
		}
	}

	out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
	got := out.(Model)
	if got.whichKey.armed || got.whichKey.shown {
		t.Fatal("an unscrollable panel must close on ctrl+d like any other key")
	}
	if got.cursor() == 0 {
		t.Fatal("ctrl+d must still page the list when the panel has nothing to scroll")
	}
}

// TestWhichKeyLeader_ScrollRewindsAfterTheTerminalWidens: the renderer clamps
// the offset on a local copy only, so widening the terminal while scrolled to
// the bottom left a stale, too-large offset on the model and the first ctrl+u
// was visually dead — it only walked the offset back to what was already on
// screen. The scroll handler now re-clamps against the current layout first.
func TestWhichKeyLeader_ScrollRewindsAfterTheTerminalWidens(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := whichKeyTestModel()
	m.width, m.height = 80, 14
	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)

	narrow, ok := m.whichKeyLayoutFor(m.whichKeyLeaderCells())
	if !ok || narrow.maxScroll == 0 {
		t.Fatalf("precondition: the panel must overflow at 80x14 (ok=%v maxScroll=%d)", ok, narrow.maxScroll)
	}
	// Scroll to the very bottom, then widen so far fewer rows are needed.
	for m.whichKey.scroll < narrow.maxScroll {
		out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
		m = out.(Model)
	}
	stale := m.whichKey.scroll
	m.width, m.height = 160, 16

	wide, ok := m.whichKeyLayoutFor(m.whichKeyLeaderCells())
	if !ok || wide.maxScroll == 0 || wide.maxScroll >= stale {
		t.Fatalf("precondition: widening must shrink maxScroll below the stale offset but keep scrolling (stale=%d, new maxScroll=%d)", stale, wide.maxScroll)
	}

	out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageUp))
	got := out.(Model).whichKey.scroll
	if want := max(wide.maxScroll-max(wide.viewRows/2, 1), 0); got != want {
		t.Fatalf("ctrl+u after a widen must step back from the clamped bottom (%d), got scroll=%d want %d", wide.maxScroll, got, want)
	}
}

// TestWhichKeyPanel_CellsBuiltOncePerFrame: the panel and the hint bar both
// need the same entries, and each build runs the whole availability catalog.
// renderView primes a one-frame cache; both consumers must read it rather than
// rebuild.
func TestWhichKeyPanel_CellsBuiltOncePerFrame(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	if primed := m.primeWhichKeyCells(); primed.whichKey.cells != nil {
		t.Fatal("no panel on screen: nothing to cache")
	}

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	primed := m.primeWhichKeyCells()
	if len(primed.whichKey.cells) == 0 {
		t.Fatal("a shown leader panel must prime the frame cache")
	}

	// A sentinel the real catalog can never produce: if either consumer
	// rebuilt instead of reading the cache, it would not see this.
	sentinel := []whichKeyCell{{key: "Z", desc: "sentinel-only-entry"}}
	fillWhichKeyDisplay(sentinel)
	m.whichKey.cells = sentinel

	bg := strings.Repeat("\n", m.height)
	if out := stripANSI(m.renderWhichKeyLeader(bg)); !strings.Contains(out, "sentinel-only-entry") {
		t.Errorf("the panel must render the frame cache:\n%s", out)
	}
	// One cell never overflows, so the hint bar must drop the scroll hint —
	// which it can only know from the cached cells, not from the real catalog.
	for _, h := range m.leaderOrExplorerHints() {
		if h.Desc == "scroll" {
			t.Error("the hint bar must read the frame cache, not rebuild the catalog")
		}
	}
}

// TestArmWhichKeyLeader_DisabledPathNeverRewindsSeq: seq is a generation
// counter whose whole job is never to repeat, so an in-flight reveal tick can
// be matched to the arming that scheduled it. The disabled branch used to zero
// the entire state — the one place that walked seq backwards. It is
// unreachable in production (handleExplorerSelectionKey returns first), but it
// must not be a trap if that ever changes.
func TestArmWhichKeyLeader_DisabledPathNeverRewindsSeq(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = false

	m := whichKeyTestModel()
	m.whichKey = whichKeyState{armed: true, shown: true, seq: 7}
	got, cmd := m.armWhichKeyLeader()
	if cmd != nil {
		t.Fatal("a disabled panel must not schedule a reveal")
	}
	if got.whichKey.armed || got.whichKey.shown {
		t.Fatal("a disabled panel must not stay armed or shown")
	}
	if got.whichKey.seq <= 7 {
		t.Fatalf("seq must keep counting up, got %d after 7", got.whichKey.seq)
	}
}
