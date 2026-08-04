package app

import (
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestWhichKeyLegend_ShowsOnlyGroupsPresentInCells is the core promise: the
// legend must never advertise a color that has nothing on screen to match it.
func TestWhichKeyLegend_ShowsOnlyGroupsPresentInCells(t *testing.T) {
	cells := []whichKeyCell{
		{key: "d", desc: "Delete", group: wkActions},
		{key: "p", desc: "Details / YAML preview", group: wkViews},
	}
	got := whichKeyPresentGroups(cells)
	want := []whichKeyGroup{wkActions, wkViews}
	if len(got) != len(want) {
		t.Fatalf("whichKeyPresentGroups = %v, want %v", got, want)
	}
	for i, g := range want {
		if got[i] != g {
			t.Fatalf("whichKeyPresentGroups = %v, want %v", got, want)
		}
	}
}

// TestWhichKeyLegend_RendersOnlyPresentGroupNames exercises the real render
// path: a two-group cell set must draw exactly those two group names and none
// of the other four declared groups.
func TestWhichKeyLegend_RendersOnlyPresentGroupNames(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.width, m.height = 80, 20

	cells := []whichKeyCell{
		{key: "d", desc: "Delete", group: wkActions},
		{key: "p", desc: "Details / YAML preview", group: wkViews},
	}
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
	for _, want := range []string{"Actions", "Views"} {
		if !strings.Contains(out, want) {
			t.Errorf("legend missing present group %q:\n%s", want, out)
		}
	}
	for _, absent := range []string{"Filter", "Selection", "Sort", "Settings"} {
		if strings.Contains(out, absent) {
			t.Errorf("legend must not name absent group %q:\n%s", absent, out)
		}
	}
}

// TestWhichKeyLegend_OmittedForGotoPopup pins the g-prefix popup's exemption:
// its cells carry no group by construction, so it must never grow a legend
// row — the popup's own tests (TestWhichKeyCells_GotoPopupStaysUngrouped)
// already pin the ungrouped styling; this pins the row budget.
func TestWhichKeyLegend_OmittedForGotoPopup(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.pendingG = true
	m.whichKey.shown = true

	cells := m.whichKeyCells()
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		t.Fatal("precondition: the goto popup must lay out")
	}
	if lay.legendRows != 0 {
		t.Fatalf("goto popup must never reserve a legend row, got legendRows=%d", lay.legendRows)
	}
	out := stripANSI(m.renderWhichKey(strings.Repeat("\n", m.height)))
	for _, g := range whichKeyGroupOrder() {
		if strings.Contains(out, string(g)) {
			t.Errorf("goto popup must not render group name %q anywhere:\n%s", g, out)
		}
	}
}

// TestWhichKeyLegend_OmittedWhenNoGroupsPresent covers cells that carry no
// group at all (e.g. a hand-built fixture in another test): no groups means
// nothing to teach, so the row must not be reserved.
func TestWhichKeyLegend_OmittedWhenNoGroupsPresent(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyEnabled = true
	m := gotoTestModel()
	m.width, m.height = 80, 20
	cells := []whichKeyCell{{key: "d", desc: "Delete"}, {key: "e", desc: "Edit"}}
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		t.Fatal("precondition: the panel must lay out")
	}
	if lay.legendRows != 0 {
		t.Fatalf("ungrouped cells must not reserve a legend row, got legendRows=%d", lay.legendRows)
	}
}

// TestWhichKeyLegend_OmittedInNoColorMode: the group styles collapse to one
// plain style in NoColor mode (whichKeyGroupStyles), so a legend naming
// colors that no longer exist would be actively misleading — it must not
// render at all, matching TestWhichKeyGroupStyles_NoColorCollapsesButStillRenders's
// "the cue is gone" premise.
func TestWhichKeyLegend_OmittedInNoColorMode(t *testing.T) {
	restoreWhichKeyGlobals(t)
	noColor := ui.ConfigNoColor
	t.Cleanup(func() {
		ui.ConfigNoColor = noColor
		ui.ApplyTheme(ui.DefaultTheme())
	})
	ui.ConfigNoColor = true
	ui.ApplyTheme(ui.DefaultTheme())
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true

	m := whichKeyTestModel()
	m.width, m.height = 120, 40
	cells := m.whichKeyLeaderCells()
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		t.Fatal("precondition: the panel must lay out")
	}
	if lay.legendRows != 0 {
		t.Fatalf("NoColor mode must not reserve a legend row, got legendRows=%d", lay.legendRows)
	}
	// A group name is not a safe substring to search for here — real entry
	// labels contain them too ("Sort next column", "Filter list"). The row
	// count is the precise signal: no legend row means no extra content row
	// beyond the entry viewport.
	out := stripANSI(m.renderWhichKeyPanel(strings.Repeat("\n", m.height), cells, 0))
	rows := whichKeyContentRows(t, out, lay.container)
	if len(rows) != lay.viewRows {
		t.Errorf("NoColor panel drew %d content rows, want exactly the %d-row entry viewport with no legend footer", len(rows), lay.viewRows)
	}
}

// TestWhichKeyLegend_ClusterLevelDropsUnavailableGroups is the cluster-picker
// scenario from the design brief: Sort and Selection require LevelResources
// or deeper (wkSortApplies, wkOnRow), so at LevelClusters neither group has a
// single entry and the legend must not name either.
func TestWhichKeyLegend_ClusterLevelDropsUnavailableGroups(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	m := whichKeyTestModel()
	m.nav.Level = model.LevelClusters
	m.setMiddleItems([]model.Item{{Name: "ctx-a"}})
	m.setCursor(0)
	m.width, m.height = 120, 40

	cells := m.whichKeyLeaderCells()
	groups := whichKeyPresentGroups(cells)
	present := map[whichKeyGroup]bool{}
	for _, g := range groups {
		present[g] = true
	}
	if present[wkSort] || present[wkSelection] {
		t.Fatalf("Sort/Selection must be absent at LevelClusters, got present groups %v", groups)
	}
	if !present[wkActions] {
		t.Fatalf("Actions must still be present at LevelClusters (Action menu is always offered), got %v", groups)
	}

	out := stripANSI(m.renderWhichKeyLeader(strings.Repeat("\n", m.height)))
	for _, absent := range []string{"Sort", "Selection"} {
		if strings.Contains(out, absent) {
			t.Errorf("LevelClusters legend must not name %q:\n%s", absent, out)
		}
	}
}

// TestWhichKeyLegend_NeverMistakenForAnEntryByReachability guards the
// interaction the review brief called out by name: a legend row must not
// satisfy the "every available entry is reachable via scrolling" invariant
// for a label it happens to share a substring with, and conversely the
// reachability scan must still find every real entry once the legend has
// claimed a row out of the viewport budget.
func TestWhichKeyLegend_NeverMistakenForAnEntryByReachability(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyEnabled = true
	ui.ConfigWhichKeyLeaderDelayMs = 0
	m := whichKeyTestModel()
	m.width, m.height = 80, 14

	want := map[string]bool{}
	for _, a := range m.availableWhichKeyActions() {
		want[a.Label] = true
	}

	out, _ := m.handleExplorerKey(leaderKey())
	m = out.(Model)
	cells := m.whichKeyLeaderCells()
	lay, ok := m.whichKeyLayoutFor(cells)
	if !ok {
		t.Fatal("precondition: the panel must lay out at 80x14")
	}
	if lay.legendRows == 0 {
		t.Fatal("precondition: this catalog must trigger the legend at 80x14")
	}

	bg := strings.Repeat("\n", m.height)
	var rendered strings.Builder
	rendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
	for step := 0; m.whichKey.scroll < lay.maxScroll; step++ {
		if step > lay.bodyRows {
			t.Fatalf("ctrl+d never reached the end (%d of %d)", m.whichKey.scroll, lay.maxScroll)
		}
		out, _ = m.handleExplorerKey(keyMsg(ui.ActiveKeybindings.PageDown))
		m = out.(Model)
		rendered.WriteString("\n")
		rendered.WriteString(stripANSI(m.renderWhichKeyLeader(bg)))
	}
	for i, c := range cells {
		col := i / lay.grid.rowN
		drawn := c.keyText() + " " + ui.Truncate(c.desc, lay.grid.descW[col])
		if !strings.Contains(rendered.String(), drawn) {
			t.Errorf("%q never appears at any scroll offset — the legend row must not have starved it", drawn)
		}
	}
}
