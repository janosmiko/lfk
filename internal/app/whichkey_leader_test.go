package app

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func TestSpace_TogglesSelectionAndArms(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300
	m := whichKeyTestModel()

	out, _ := m.handleExplorerKey(spaceKey())
	got := out.(Model)
	if len(got.selectedItems) != 1 {
		t.Fatalf("space must still toggle selection; selected=%d", len(got.selectedItems))
	}
	if !got.whichKey.armed {
		t.Fatal("space must arm the which-key leader")
	}
	if got.whichKey.shown {
		t.Fatal("panel must stay hidden until the delay elapses")
	}
}

func TestSpace_ZeroDelayShowsImmediately(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	out, _ := m.handleExplorerKey(spaceKey())
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
	armed, _ := m.handleExplorerKey(spaceKey())
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
	m.setCursor(0)

	armedOut, _ := m.handleExplorerKey(spaceKey())
	armed := armedOut.(Model)
	if !armed.whichKey.armed {
		t.Fatal("precondition: space must arm")
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

func TestSpace_DoesNotArmWhileFiltering(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.filterActive = true
	out, _ := m.Update(spaceKey())
	if out.(Model).whichKey.armed {
		t.Fatal("typing a space into the filter must not arm the leader")
	}
}

// TestWhichKeyLeader_PageAdvancesAndWraps pins the USER DECISION paging
// behavior: repeated space (while already armed) pages forward, and wraps
// back to page 0 after the last page — using the real catalog at 80x24, the
// size the design was validated against (only ~24 of ~49 entries fit on one
// page, so multiple pages are expected here, not a contrived fixture).
func TestWhichKeyLeader_PageAdvancesAndWraps(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(spaceKey())
	m = out.(Model)
	_, idx0, count := m.whichKeyLeaderPage()
	if idx0 != 0 {
		t.Fatalf("first arming must land on page 0, got %d", idx0)
	}
	if count < 2 {
		t.Fatalf("test assumes the full catalog needs multiple pages at 80x24; got %d page(s)", count)
	}

	for i := 1; i < count+2; i++ {
		out, _ = m.handleExplorerKey(spaceKey())
		m = out.(Model)
		_, idx, c := m.whichKeyLeaderPage()
		if c != count {
			t.Fatalf("press %d: page count changed mid-sequence: got %d, want %d", i, c, count)
		}
		want := i % count
		if idx != want {
			t.Fatalf("press %d: page = %d, want %d (must wrap at %d pages)", i, idx, want, count)
		}
	}
}

// TestWhichKeyLeader_PageAdvanceDoesNotReHideAtDefaultDelay is CRITICAL-1
// from review round 1: at the shipped default delay (300ms), a page-advance
// press must not re-hide an already-revealed panel. armWhichKeyLeader used
// to unconditionally set shown=false whenever the delay was > 0, including
// on a repeat press — so every page advance blinked the panel off for
// another full delay before it reappeared. Only a FRESH arm (not already
// armed) may hide the panel; a repeat press while already armed and shown
// must leave shown alone.
func TestWhichKeyLeader_PageAdvanceDoesNotReHideAtDefaultDelay(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	// Fresh arm: must not show immediately at a nonzero delay.
	out, _ := m.handleExplorerKey(spaceKey())
	m = out.(Model)
	if m.whichKey.shown {
		t.Fatal("precondition: fresh arm must not show immediately at 300ms delay")
	}

	// Simulate the delay elapsing: the scheduled tick fires for this seq.
	out, _, _ = m.updateEasterEggMsg(whichKeyLeaderTickMsg{seq: m.whichKey.seq})
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("precondition: the tick must reveal the panel")
	}

	// Page-advance press: the panel is already shown and must stay shown —
	// not blink off for another 300ms before the next page appears.
	out, _ = m.handleExplorerKey(spaceKey())
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("CRITICAL-1: a page-advance press must not hide an already-shown panel")
	}
	if m.whichKey.page != 1 {
		t.Fatalf("the press must still advance the page: got %d, want 1", m.whichKey.page)
	}
}

// TestWhichKeyLeader_PageResetsOnDisarmAndRearm covers both halves of "page
// lives on whichKeyState and resets whenever the leader disarms": disarming
// (via esc) zeroes the stored page, and arming fresh afterward starts back on
// page 0 rather than continuing where the previous arming left off.
func TestWhichKeyLeader_PageResetsOnDisarmAndRearm(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(spaceKey())
	m = out.(Model)
	out, _ = m.handleExplorerKey(spaceKey()) // second press: advance off page 0
	m = out.(Model)
	if m.whichKey.page == 0 {
		t.Fatal("precondition: a second press while armed should have advanced the page")
	}

	out, _ = m.handleExplorerKey(tea.KeyPressMsg{Code: tea.KeyEscape})
	m = out.(Model)
	if m.whichKey.page != 0 {
		t.Fatalf("disarm (esc) must reset the stored page to 0, got %d", m.whichKey.page)
	}

	out, _ = m.handleExplorerKey(spaceKey())
	m = out.(Model)
	_, idx, _ := m.whichKeyLeaderPage()
	if idx != 0 {
		t.Fatalf("re-arming from scratch must start on page 0, got %d", idx)
	}
}

// TestSpace_TogglesSelectionOnEveryPageAdvancingPress guards the load-bearing
// requirement: pressing space to page through the panel must never skip a
// selection toggle, even though the same press also advances the page.
func TestSpace_TogglesSelectionOnEveryPageAdvancingPress(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 24
	m.setMiddleItems([]model.Item{
		{Name: "p1", Kind: "Pod", Namespace: "default"},
		{Name: "p2", Kind: "Pod", Namespace: "default"},
		{Name: "p3", Kind: "Pod", Namespace: "default"},
	})
	m.setCursor(0)

	for i := 1; i <= 3; i++ {
		out, _ := m.handleExplorerKey(spaceKey())
		m = out.(Model)
		if len(m.selectedItems) != i {
			t.Fatalf("press %d: selected=%d, want %d — space must toggle selection on every press, including page-advancing ones", i, len(m.selectedItems), i)
		}
		if !m.whichKey.armed {
			t.Fatalf("press %d: the leader must stay armed across repeated space presses", i)
		}
	}
}

// TestSpace_JNavigationMultiSelectsWithoutFlashingPanelAtDefaultDelay is the
// scenario the whole design rests on: "space j space j" must still
// multi-select two rows, and at the 300ms default delay the panel must never
// appear, because j is not the leader key and disarms it (via
// disarmWhichKeyLeader) well before any scheduled reveal tick could fire.
func TestSpace_JNavigationMultiSelectsWithoutFlashingPanelAtDefaultDelay(t *testing.T) {
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
		if m.whichKey.shown {
			t.Fatal("panel must not appear before the 300ms delay elapses")
		}
		out, _ = m.handleExplorerKey(downKey)
		m = out.(Model)
		if m.whichKey.armed || m.whichKey.shown {
			t.Fatal("j must disarm the leader immediately, before any delayed reveal")
		}
	}
	if len(m.selectedItems) != 2 {
		t.Fatalf("space j space j must multi-select exactly two rows; selected=%d", len(m.selectedItems))
	}
}

// TestWhichKeyLeaderPage_Page1HighestPriorityAt80x24 pins the USER DECISION
// group order at the terminal size it was validated against: page 1 must open
// on Actions, the highest-priority group in whichKeyGroupOrder.
func TestWhichKeyLeaderPage_Page1HighestPriorityAt80x24(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	groups, idx, count := m.whichKeyLeaderPage()
	if idx != 0 {
		t.Fatalf("a freshly-queried page index must be 0, got %d", idx)
	}
	if len(groups) == 0 {
		t.Fatal("page 1 must not be empty at 80x24")
	}
	if groups[0].Title != string(wkActions) {
		t.Fatalf("page 1's first group must be %q (highest priority), got %q", wkActions, groups[0].Title)
	}
	if count < 2 {
		t.Fatalf("expected the full catalog to need multiple pages at 80x24, got %d", count)
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
func TestWhichKeyLeaderPage_NeverEmptyBoxAtShortHeights(t *testing.T) {
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

// TestWhichKeyLeaderPage_AllEntriesReachableViaPaging is IMPORTANT-3 from
// review round 1: paginateWhichKeyGroups used to give an oversized group its
// own solo page without ever fitting it, so renderWhichKeyPanel's "+N more"
// fallback silently swallowed the rest of that group at RENDER time — and
// because paging only ever moved between groups (never split one), those
// entries were permanently unreachable by any number of space presses, even
// though whichKeyLeaderPage's own returned (unfit) groups still technically
// contained them. This asserts reachability the way a user experiences it:
// by scanning the actual rendered text of every page, not the raw group
// data the renderer hasn't yet truncated.
func TestWhichKeyLeaderPage_AllEntriesReachableViaPaging(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 80, 14
	m.whichKey.armed = true
	m.whichKey.shown = true

	want := map[string]bool{}
	for _, a := range m.availableWhichKeyActions() {
		want[a.Label] = true
	}

	_, _, count := m.whichKeyLeaderPage()
	if count < 2 {
		t.Fatalf("test assumes the full catalog needs multiple pages at 80x14; got %d", count)
	}
	var allRendered strings.Builder
	for p := range count {
		m.whichKey.page = p
		bg := strings.Repeat("\n", m.height)
		allRendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
		allRendered.WriteString("\n")
	}
	rendered := allRendered.String()
	for label := range want {
		if !strings.Contains(rendered, label) {
			t.Errorf("IMPORTANT-3: %q never appears in any of the %d rendered pages — unreachable via paging", label, count)
		}
	}
}

// TestWhichKeyLeader_HintBarGatesOnShownNotArmed is IMPORTANT-4 from review
// round 1: the hint bar used to switch to the leader's "space: more / esc:
// close" hints as soon as the leader armed, not once the panel actually
// appeared. At the shipped default delay (300ms) that meant EVERY ordinary
// multi-select press replaced the whole status bar — including the selected
// count, sort indicator, and position counter — for the length of the delay,
// even though the panel itself was invisible the entire time.
func TestWhichKeyLeader_HintBarGatesOnShownNotArmed(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300
	m := whichKeyTestModel()
	m.width, m.height = 80, 24

	out, _ := m.handleExplorerKey(spaceKey())
	m = out.(Model)
	if m.whichKey.shown {
		t.Fatal("precondition: fresh arm must not be shown yet at 300ms delay")
	}
	if hint := stripANSI(m.statusBar()); strings.Contains(hint, "space: more") {
		t.Fatalf("IMPORTANT-4: hint bar switched to leader hints before the panel is shown:\n%s", hint)
	}

	out, _, _ = m.updateEasterEggMsg(whichKeyLeaderTickMsg{seq: m.whichKey.seq})
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("precondition: the tick must reveal the panel")
	}
	if hint := stripANSI(m.statusBar()); !strings.Contains(hint, "space: more") {
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

	armedOut, _ := m.handleExplorerKey(spaceKey())
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

	armedOut, _ := m.handleExplorerKey(spaceKey())
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

// TestWhichKeyLeaderPage_IndicatorAlwaysShownWhenMultiplePages guards the
// other half of IMPORTANT-3: once entries are reachable via splitting
// (rather than dropped into "+N more"), every page of a multi-page panel
// must show its page indicator — the whole point of paging is defeated if
// the user can't tell there's more to see.
func TestWhichKeyLeaderPage_IndicatorAlwaysShownWhenMultiplePages(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 80, 14
	m.whichKey.armed = true
	m.whichKey.shown = true

	_, _, count := m.whichKeyLeaderPage()
	if count < 2 {
		t.Fatalf("test assumes multiple pages at 80x14; got %d", count)
	}
	for p := range count {
		m.whichKey.page = p
		bg := strings.Repeat("\n", m.height)
		out := stripANSI(m.renderWhichKeyLeader(bg))
		want := fmt.Sprintf("(%d/%d)", p+1, count)
		if !strings.Contains(out, want) {
			t.Errorf("IMPORTANT-3: page %d/%d missing its indicator %q:\n%s", p+1, count, want, out)
		}
	}
}

// TestWhichKeyLeaderPage_NoIndicatorOnSinglePage is a review round 1 minor:
// a single-page panel has nothing to page to, so "(1/1)" would falsely
// promise more content than exists.
func TestWhichKeyLeaderPage_NoIndicatorOnSinglePage(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.width, m.height = 220, 220 // plenty of room for the whole catalog on one page
	m.whichKey.armed = true
	m.whichKey.shown = true

	_, _, count := m.whichKeyLeaderPage()
	if count != 1 {
		t.Fatalf("test assumes a single page at 220x220; got %d", count)
	}
	bg := strings.Repeat("\n", m.height)
	out := stripANSI(m.renderWhichKeyLeader(bg))
	if strings.Contains(out, "(1/1)") {
		t.Fatalf("a single page must not show a page indicator:\n%s", out)
	}
}
