package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestStatusBar_GotoPopupKeepsExplorerHints pins the hint bar to the leader,
// not to the shared whichKeyState. The g-prefix goto popup sets shown too, but
// handleGotoChord swallows the leader key as an unregistered chord, so
// advertising "?: more" there promises paging that cannot happen.
func TestStatusBar_GotoPopupKeepsExplorerHints(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	m.pendingG = true
	m.whichKey = whichKeyState{shown: true}

	bar := stripANSI(m.statusBar())
	if strings.Contains(bar, "esc: close") {
		t.Fatalf("goto popup must not show the leader hints; got %q", bar)
	}
	if !strings.Contains(bar, "namespace") {
		t.Fatalf("goto popup must keep the normal explorer hints; got %q", bar)
	}
}

// TestStatusBar_LeaderPanelKeepsSelectedCountChip guards the one indicator the
// user still needs while the panel is open: the panel covers list rows, not
// the status bar, so suppressing the chip group (as an early return once did)
// hides the selected count for no reason.
func TestStatusBar_LeaderPanelKeepsSelectedCountChip(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := basePush80Model()
	m.nav.Level = model.LevelResources
	items := make([]model.Item, 5)
	for i := range items {
		items[i] = model.Item{Name: fmt.Sprintf("p%d", i), Kind: "Pod", Namespace: "default"}
	}
	m.setMiddleItems(items)
	m.setCursor(0)
	for _, it := range items {
		m.selectedItems[selectionKey(it)] = true
	}

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("panel must be shown at a zero delay")
	}
	for step := 1; step <= 3; step++ {
		bar := stripANSI(m.statusBar())
		if !strings.Contains(bar, "esc: close") {
			t.Fatalf("step %d: leader hints must still render; got %q", step, bar)
		}
		chip := fmt.Sprintf("%d selected", len(m.selectedItems))
		if !strings.Contains(bar, chip) {
			t.Fatalf("step %d: status bar must keep the %q chip while the panel is shown; got %q", step, chip, bar)
		}
		out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
		m = out.(Model)
	}
	if len(m.selectedItems) != 5 {
		t.Fatalf("scrolling must not change the selection; got %d, want 5", len(m.selectedItems))
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

// TestLeaderThenEsc_BeforeTickStillClearsSelection pins esc's normal explorer
// job during a configured delay window. The panel is not drawn yet, so
// consuming esc there steals it with no on-screen feedback.
func TestLeaderThenEsc_BeforeTickStillClearsSelection(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 300

	m := whichKeyTestModel()
	out, _ := m.handleExplorerKey(spaceKey()) // something for esc to clear
	m = out.(Model)
	if !m.hasSelection() {
		t.Fatal("precondition: space must have selected the row")
	}
	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.armed || m.whichKey.shown {
		t.Fatalf("the leader must arm without showing at a 300ms delay; armed=%v shown=%v", m.whichKey.armed, m.whichKey.shown)
	}

	out, _ = m.handleExplorerKey(keyMsg("esc"))
	m = out.(Model)
	if m.whichKey.armed {
		t.Error("esc must disarm the leader")
	}
	if m.hasSelection() {
		t.Error("esc must still clear the selection while the panel is invisible")
	}
}

// TestLeaderThenEsc_ClosesVisiblePanelOnly is the other half of the gate: once
// the panel is actually on screen, esc closing it is the visible effect and
// must not also run the explorer's own esc action.
func TestLeaderThenEsc_ClosesVisiblePanelOnly(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0

	m := whichKeyTestModel()
	out, _ := m.handleExplorerKey(spaceKey()) // something esc could wrongly clear
	m = out.(Model)
	out, _ = m.handleExplorerKey(leaderKey())
	m = out.(Model)
	if !m.whichKey.shown {
		t.Fatal("a zero delay must reveal the panel immediately")
	}

	out, _ = m.handleExplorerKey(keyMsg("esc"))
	m = out.(Model)
	if m.whichKey.armed || m.whichKey.shown {
		t.Error("esc must close the panel")
	}
	if !m.hasSelection() {
		t.Error("esc closing a visible panel must not also clear the selection")
	}
}
