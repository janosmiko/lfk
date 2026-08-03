package app

import (
	"fmt"
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

func TestWhichKeyLeaderGroups_AreOrderedAndNonEmpty(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	groups := whichKeyTestModel().whichKeyLeaderGroups()
	if len(groups) == 0 {
		t.Fatal("a resource row must yield at least one group")
	}
	order := whichKeyGroupOrder()
	pos := 0
	for _, g := range groups {
		if len(g.Cells) == 0 {
			t.Fatalf("group %q must not be emitted with no cells", g.Title)
		}
		for pos < len(order) && string(order[pos]) != g.Title {
			pos++
		}
		if pos == len(order) {
			t.Fatalf("group %q is out of the declared order", g.Title)
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

// TestWhichKeyLeader_RepeatPressTogglesClosed pins the USER DECISION that
// replaced paging: with the panel scrolling there is nothing for a repeat press
// to advance to, so the leader key toggles — press to open, press again to
// close, press again to reopen at the top.
func TestWhichKeyLeader_RepeatPressTogglesClosed(t *testing.T) {
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

	m = m.scrollWhichKey(m.whichKeyLeaderGroups(), true)
	if m.whichKey.scroll == 0 {
		t.Fatal("precondition: the full catalog must be scrollable at 80x24")
	}

	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if m.whichKey.armed || m.whichKey.shown {
		t.Fatal("a repeat press must close the panel, not advance it")
	}

	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("a third press must reopen the panel")
	}
	if m.whichKey.scroll != 0 {
		t.Fatalf("reopening must start at the top, got scroll=%d", m.whichKey.scroll)
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
	lay, ok := m.whichKeyLayoutFor(m.whichKeyLeaderGroups())
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

// TestWhichKeyLeader_OpensOnHighestPriorityGroupAt80x24 pins the USER DECISION
// group order at the terminal size it was validated against: the unscrolled
// panel must open on Actions, the highest-priority group in whichKeyGroupOrder.
func TestWhichKeyLeader_OpensOnHighestPriorityGroupAt80x24(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 80, 24
	m.whichKey.armed = true
	m.whichKey.shown = true

	groups := m.whichKeyLeaderGroups()
	if len(groups) == 0 {
		t.Fatal("the panel must not be empty at 80x24")
	}
	if groups[0].Title != string(wkActions) {
		t.Fatalf("the first group must be %q (highest priority), got %q", wkActions, groups[0].Title)
	}
	out := stripANSI(m.renderWhichKeyLeader(strings.Repeat("\n", m.height)))
	if !strings.Contains(out, string(wkActions)) {
		t.Fatalf("the unscrolled panel must open on %q:\n%s", wkActions, out)
	}
	lay, _ := m.whichKeyLayoutFor(groups)
	if lay.maxScroll == 0 {
		t.Fatal("expected the full catalog to overflow at 80x24")
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
		if !strings.Contains(out, "Actions") {
			t.Errorf("CRITICAL-2: h=%d rendered a box with no real content (empty except chrome/footer):\n%s", h, out)
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
		lay, ok := m.whichKeyLayoutFor(m.whichKeyLeaderGroups())
		if !ok {
			t.Fatalf("%dx%d: the panel must lay out", size[0], size[1])
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
		for label := range want {
			if !strings.Contains(rendered.String(), ui.Truncate(label, lay.grid.descW)) {
				t.Errorf("%dx%d: %q never appears at any scroll offset — unreachable", size[0], size[1], label)
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
		if lay, _ := m.whichKeyLayoutFor(m.whichKeyLeaderGroups()); lay.maxScroll == 0 {
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
		if lay, _ := m.whichKeyLayoutFor(m.whichKeyLeaderGroups()); lay.maxScroll != 0 {
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
