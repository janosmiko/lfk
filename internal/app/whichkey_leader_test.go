package app

import (
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
