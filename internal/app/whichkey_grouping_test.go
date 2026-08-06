package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// The leader key's second state: ungrouped, pure key order. See the USER
// DECISION on armWhichKeyLeader and sortWhichKeyCells.

func TestSortWhichKeyCells_UngroupedIgnoresGroupAndOrder(t *testing.T) {
	build := func() []whichKeyCell {
		return []whichKeyCell{
			{key: "w", desc: "Watch", group: wkSettings},
			{key: "<", desc: "Sort previous", group: wkSort, order: 1},
			{key: "a", desc: "Toggle select", group: wkSelection},
			{key: ">", desc: "Sort next", group: wkSort, order: 2},
			{key: "y", desc: "Copy name", group: wkActions},
			{key: "ctrl+d", desc: "Diff", group: wkSelection},
			{key: "P", desc: "Preview", group: wkViews},
		}
	}
	got := build()
	sortWhichKeyCells(got, false)

	// The same cells stripped of every group and order must sort identically:
	// that is what "pure key ordering" means.
	bare := build()
	for i := range bare {
		bare[i].group = ""
		bare[i].order = 0
	}
	sortWhichKeyCells(bare, false)
	for i := range got {
		if got[i].key != bare[i].key {
			t.Fatalf("ungrouped order must not read group or order:\n got %v\nwant %v", keysOf(got), keysOf(bare))
		}
	}

	// And it must actually differ from grouped, or the toggle shows nothing.
	grouped := build()
	sortWhichKeyCells(grouped, true)
	if slices.Equal(keysOf(got), keysOf(grouped)) {
		t.Fatalf("grouped and ungrouped produced the same order (%v); the toggle would be invisible", keysOf(got))
	}
}

func keysOf(cells []whichKeyCell) []string {
	out := make([]string, 0, len(cells))
	for _, c := range cells {
		out = append(out, c.key)
	}
	return out
}

// The g-prefix goto popup carries no groups and no order overrides, so the
// toggle must not move a single one of its rows.
func TestWhichKeyCells_GotoPopupIsUnaffectedByTheGrouping(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := gotoTestModel()
	m.whichKey.grouping = wkGroupOn
	grouped := keysOf(m.whichKeyCells())
	m.whichKey.grouping = wkGroupOff
	ungrouped := keysOf(m.whichKeyCells())
	if !slices.Equal(grouped, ungrouped) {
		t.Fatalf("the goto popup has no groups; the toggle must not reorder it:\n grouped %v\nungrouped %v", grouped, ungrouped)
	}
}

// The panel keeps its per-group description colors and its legend in BOTH
// modes: ungrouped, the order no longer says which category an entry is in, so
// the color is the only thing left that does.
func TestWhichKeyLeader_UngroupedKeepsTheGroupColorsAndLegend(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigNoColor = false

	m := whichKeyTestModel()
	m.whichKey.grouping = wkGroupOff
	cells := m.whichKeyLeaderCells()
	if !whichKeyWantsLegend(cells) {
		t.Fatal("ungrouped cells must still carry their group, so the legend still renders")
	}
	if len(whichKeyPresentGroups(cells)) < 2 {
		t.Fatalf("the legend must still name every group on screen; got %v", whichKeyPresentGroups(cells))
	}
}

// which_key_grouped sets the STARTUP default. A model that has never toggled
// reads it live rather than a copy taken at construction.
func TestWhichKeyGrouped_FollowsTheConfigUntilToggled(t *testing.T) {
	restoreWhichKeyGlobals(t)
	m := whichKeyTestModel()

	ui.ConfigWhichKeyGrouped = false
	if m.whichKeyGrouped() {
		t.Error("an untouched model must follow which_key_grouped: false")
	}
	ui.ConfigWhichKeyGrouped = true
	if !m.whichKeyGrouped() {
		t.Error("an untouched model must follow which_key_grouped: true")
	}

	// Once toggled, the session state wins and the config is no longer read.
	m = m.toggleWhichKeyGrouping()
	if m.whichKeyGrouped() {
		t.Error("the toggle must flip away from the configured default")
	}
	ui.ConfigWhichKeyGrouped = false
	if m.whichKeyGrouped() {
		t.Error("a toggled model must keep its own state, not fall back to the config")
	}
}

// The leader key no longer closes the panel, so the hint bar has to say what it
// does now — and esc, the way out, must survive the width squeeze first.
func TestWhichKeyLeader_HintBarAdvertisesTheOrderToggle(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	m := whichKeyTestModel()
	m.width, m.height = 160, 40
	hints := m.whichKeyPopupHints(m.whichKeyLeaderCells(), true)
	keys := make([]string, 0, len(hints))
	descs := make([]string, 0, len(hints))
	for _, h := range hints {
		keys = append(keys, h.Key)
		descs = append(descs, h.Desc)
	}
	if !slices.Contains(keys, ui.ActiveKeybindings.WhichKeyLeader) {
		t.Errorf("the leader panel must advertise its own key; got %v", keys)
	}
	if !slices.Contains(descs, "close") {
		t.Errorf("esc/close must stay in the hints; got %v", descs)
	}
	if last := descs[len(descs)-1]; !strings.Contains(last, "order") {
		t.Errorf("the order toggle must sit last so a narrow bar drops it before close; got %v", descs)
	}

	// The goto popup does not toggle anything, so it must not claim it does.
	goto1 := m.whichKeyPopupHints(m.whichKeyCells(), false)
	for _, h := range goto1 {
		if strings.Contains(h.Desc, "order") {
			t.Errorf("the goto popup has no order toggle; got %v", goto1)
		}
	}
}
