package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestStatusBar_GotoPopupShowsOwnHints covers the user-reported bug: pressing
// g opened the goto popup but the hint bar kept advertising the normal
// explorer keymap (navigate/move/etc), none of which the popup honours while
// it's up — handleGotoChord swallows every key but the scroll pair and a
// registered chord. The popup gets its own hints instead of either borrowing
// the leader's or falling through to the explorer's.
func TestStatusBar_GotoPopupShowsOwnHints(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	m.pendingG = true
	m.whichKey = whichKeyState{shown: true}

	bar := stripANSI(m.statusBar())
	if !strings.Contains(bar, "esc: close") {
		t.Fatalf("goto popup must advertise esc: close; got %q", bar)
	}
	if strings.Contains(bar, "navigate") {
		t.Fatalf("goto popup must not show the normal explorer keymap; got %q", bar)
	}
}

// TestStatusBar_GotoPopupScrollHintOnlyWhenOverflowing mirrors
// TestWhichKeyLeader_ScrollHintOnlyWhenOverflowing for the goto popup: it
// shares the leader panel's scrolling viewport (handleGotoChord routes
// PageDown/PageUp to it while shown), so the hint bar must say so only when
// the goto catalog actually overflows, never as a blanket claim.
func TestStatusBar_GotoPopupScrollHintOnlyWhenOverflowing(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	kb := ui.DefaultKeybindings()
	scrollHint := kb.PageDown + "/" + kb.PageUp

	t.Run("overflowing", func(t *testing.T) {
		ui.ActiveKeybindings = kb
		m := whichKeyTestModel()
		m.width, m.height = 80, 10
		m.pendingG = true
		m.whichKey = whichKeyState{shown: true}
		if lay, _ := m.whichKeyLayoutFor(m.whichKeyCells()); lay.maxScroll == 0 {
			t.Fatal("precondition: the goto catalog must overflow at 80x10")
		}
		if bar := stripANSI(m.statusBar()); !strings.Contains(bar, scrollHint) {
			t.Fatalf("an overflowing goto popup must advertise the scroll keys; got %q", bar)
		}
	})

	t.Run("fits", func(t *testing.T) {
		ui.ActiveKeybindings = kb
		m := whichKeyTestModel()
		m.width, m.height = 120, 40
		m.pendingG = true
		m.whichKey = whichKeyState{shown: true}
		if lay, _ := m.whichKeyLayoutFor(m.whichKeyCells()); lay.maxScroll != 0 {
			t.Fatal("precondition: the goto catalog must fit at 120x40")
		}
		bar := stripANSI(m.statusBar())
		if strings.Contains(bar, scrollHint) {
			t.Fatalf("a goto popup that fits must not advertise the scroll keys; got %q", bar)
		}
		if !strings.Contains(bar, "esc: close") {
			t.Fatalf("the close hint must always render while the popup is up; got %q", bar)
		}
	})
}

// TestStatusBar_GotoPopupDuringDelayKeepsExplorerHints is the goto-popup
// half of TestWhichKeyLeader_HintBarGatesOnShownNotArmed: pendingG arms
// immediately, but with which_key_delay_ms configured the popup itself is not
// drawn yet. The hint bar must not flip to the popup's hints (and drop the
// explorer keymap) for a popup nothing on screen shows.
func TestStatusBar_GotoPopupDuringDelayKeepsExplorerHints(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	m.pendingG = true
	m.whichKey = whichKeyState{shown: false} // armed but not yet revealed

	bar := stripANSI(m.statusBar())
	if strings.Contains(bar, "esc: close") {
		t.Fatalf("goto popup hints must not show before the popup is drawn; got %q", bar)
	}
	if !strings.Contains(bar, "navigate") {
		t.Fatalf("explorer hints must stay up while the popup is not yet visible; got %q", bar)
	}
}

// TestStatusBar_GotoPopupKeepsSelectedCountChip is the goto-popup half of
// TestStatusBar_LeaderPanelKeepsSelectedCountChip: the popup covers list
// rows, not the status bar, so the selected-count chip on the right must
// keep rendering alongside the popup's own hints on the left.
func TestStatusBar_GotoPopupKeepsSelectedCountChip(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	items := make([]model.Item, 3)
	for i := range items {
		items[i] = model.Item{Name: fmt.Sprintf("p%d", i), Kind: "Pod", Namespace: "default"}
	}
	m.setMiddleItems(items)
	m.setCursor(0)
	for _, it := range items {
		m.selectedItems[selectionKey(it)] = true
	}
	m.pendingG = true
	m.whichKey = whichKeyState{shown: true}

	bar := stripANSI(m.statusBar())
	if !strings.Contains(bar, "esc: close") {
		t.Fatalf("goto popup hints must still render; got %q", bar)
	}
	chip := fmt.Sprintf("%d selected", len(m.selectedItems))
	if !strings.Contains(bar, chip) {
		t.Fatalf("status bar must keep the %q chip while the goto popup is shown; got %q", chip, bar)
	}
}

// TestAvailableWhichKeyActions_UnionReadOnlySourceHidesDestructive covers the
// union case readOnlyForContext short-circuits on: nav.Context is the union
// sentinel, so passing it back in always reads writable. The destructive
// entries must resolve read-only against the row's own source cluster.
func TestAvailableWhichKeyActions_UnionReadOnlySourceHidesDestructive(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := basePush80Model()
	m.nav.Level = model.LevelResources
	m.unionMode = true
	m.nav.Context = UnionContextSentinel
	m.unionContexts = []string{"writable-ctx", "ro-ctx"}
	m.contextROOverrides = map[string]bool{"ro-ctx": true}
	m.setMiddleItems([]model.Item{{Name: "p1", Kind: "Pod", Namespace: "default", ClusterName: "ro-ctx"}})
	m.setCursor(0)

	labels := whichKeyLabels(m)
	for _, blocked := range []string{"Delete", "Force delete"} {
		if slices.Contains(labels, blocked) {
			t.Errorf("%q must not be offered on a row whose source cluster is read-only; got %v", blocked, labels)
		}
	}
}

// TestLeaderKey_DoesNotClearSelection pins that opening the keymaps overlay
// is a pure overlay trigger: it must not touch the explorer's selection, the
// job esc still owns.
func TestLeaderKey_DoesNotClearSelection(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	out, _ := m.handleExplorerKey(spaceKey())
	m = out.(Model)
	if !m.hasSelection() {
		t.Fatal("precondition: space must have selected the row")
	}

	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.hasSelection() {
		t.Error("opening the keymaps overlay must not clear the selection")
	}
}
