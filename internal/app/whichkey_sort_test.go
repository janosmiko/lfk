package app

import (
	"slices"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

func wkSortedKeys(keys ...string) []string {
	cells := make([]whichKeyCell, len(keys))
	for i, k := range keys {
		cells[i] = whichKeyCell{key: k, desc: "d"}
	}
	sortWhichKeyCells(cells, true)
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = c.key
	}
	return out
}

// TestSortWhichKeyCells_PortsNeovimOrdering covers each sorter in the ported
// chain (view.lua:20-51 with config.lua's `sort`, plus the appended natural and
// case sorters) in isolation and then together.
//
// Two cases below changed from their pre-modifier-tier expectations, BY
// DESIGN, now that the modifier tier (wkModTier) outranks alphanum/natural/
// case: a modifier chord no longer competes with plain-key punctuation on
// alphanum/mod footing — it is simply a later tier, full stop.
//   - "alphanum before everything else": "ctrl+a" used to sort ahead of the
//     bare punctuation "!" and "/" (the old `mod` sorter ranked any chord
//     ahead of plain punctuation). It is tier 1 now, so it sorts after every
//     tier-0 (unmodified) entry, punctuation included.
//   - "all four together": same reason — "ctrl+a"/"ctrl+space" now trail
//     every unmodified entry, including "."  and "?".
//
// TestSortWhichKeyCells_ModifierTierOrdering below is the dedicated coverage
// for the new tier itself; this table keeps covering alphanum/natural/case.
func TestSortWhichKeyCells_PortsNeovimOrdering(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"alphanum before punctuation, within the same modifier tier",
			[]string{"/", "y", "ctrl+a", "!", "b"},
			[]string{"b", "y", "!", "/", "ctrl+a"},
		},
		{
			"unmodified punctuation still sorts ahead of any modifier tier",
			[]string{"@", "ctrl+y", "\\", "alt+b"},
			[]string{"@", "\\", "ctrl+y", "alt+b"},
		},
		{
			"digit runs compare numerically",
			[]string{"f10", "f2", "f1", "10", "2"},
			[]string{"2", "10", "f1", "f2", "f10"},
		},
		{
			"lowercase before uppercase on a tie",
			[]string{"D", "d", "Y", "y"},
			[]string{"d", "D", "y", "Y"},
		},
		{
			"all four together",
			[]string{"Y", "ctrl+space", ".", "y", "f1", "a", "ctrl+a", "?"},
			[]string{"a", "f1", "y", "Y", ".", "?", "ctrl+a", "ctrl+space"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := wkSortedKeys(tc.in...); !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestSortWhichKeyCells_ModifierTierOrdering is the dedicated coverage for
// CHANGE 1 (USER DECISION): within a group, plain keys sort first, then
// ctrl-only chords, then alt-only chords, then ctrl+alt chords together, then
// everything else that carries a modifier the user didn't name a tier for
// (shift alone, meta/super, ctrl+shift, shift+alt+ctrl, ...) as a catch-all
// tier after ctrl+alt.
func TestSortWhichKeyCells_ModifierTierOrdering(t *testing.T) {
	in := []string{
		"ctrl+alt+y", "shift+x", "b", "alt+z", "ctrl+shift+x",
		"ctrl+b", "meta+k", "?", "alt+ctrl+q", "super+k",
	}
	// tier0: b, ? (alphanum before punctuation)
	// tier1 (ctrl only): ctrl+b
	// tier2 (alt only): alt+z
	// tier3 (ctrl+alt, either spelling): natural-key compares the raw string,
	// so "alt+ctrl+q" (starts 'a') sorts ahead of "ctrl+alt+y" (starts 'c')
	// tier4 (catch-all), same natural-key raw-string compare:
	// "ctrl+shift+x" ('c') < "meta+k" ('m') < "shift+x" ('sh') < "super+k" ('su')
	want := []string{
		"b", "?",
		"ctrl+b",
		"alt+z",
		"alt+ctrl+q", "ctrl+alt+y",
		"ctrl+shift+x", "meta+k", "shift+x", "super+k",
	}
	if got := wkSortedKeys(in...); !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestWkModTier_ClassifiesEveryCombination pins wkModTier directly against
// the tier boundaries the sort now uses, independent of the other sorters.
func TestWkModTier_ClassifiesEveryCombination(t *testing.T) {
	tests := []struct {
		key  string
		tier int
	}{
		{"d", 0},
		{"?", 0},
		{"f1", 0},
		{"space", 0},
		{"ctrl++", 0}, // literal "+" chord
		{"ctrl+a", 1},
		{"ctrl+space", 1},
		{"alt+b", 2},
		{"ctrl+alt+y", 3},
		{"alt+ctrl+y", 3},
		{"shift+x", 4},
		{"meta+k", 4},
		{"super+k", 4},
		{"hyper+k", 4},
		{"ctrl+shift+x", 4},
		{"shift+alt+ctrl+y", 4},
	}
	for _, tc := range tests {
		if got := wkModTier(tc.key); got != tc.tier {
			t.Errorf("wkModTier(%q) = %d, want %d", tc.key, got, tc.tier)
		}
	}
}

// TestSortWhichKeyCells_OrderIsIconModeIndependent pins that the sort reads
// whichKeyCell.key (the raw binding) and never whichKeyCell.disp (the
// rendered form fillWhichKeyDisplay fills in afterwards) — the panel must
// order identically whether icons draw "⌃L" or "ctrl+l" verbatim.
func TestSortWhichKeyCells_OrderIsIconModeIndependent(t *testing.T) {
	restoreWhichKeyGlobals(t)
	keys := []string{"y", "ctrl+l", "alt+x", "ctrl+alt+y", "shift+tab", "?", "space", "D", "d"}

	var baseline []string
	for _, mode := range []string{"ascii", "unicode", "nerdfont", "emoji"} {
		ui.IconMode = mode
		got := wkSortedKeys(keys...)
		if baseline == nil {
			baseline = got
			continue
		}
		if !slices.Equal(got, baseline) {
			t.Fatalf("icon mode %q changed the sort order: got %v, want %v (from the baseline mode)", mode, got, baseline)
		}
	}
}

// TestSortWhichKeyCells_IsDeterministic: equal ranks fall back to the raw key,
// so the panel can never reorder itself between two identical frames.
func TestSortWhichKeyCells_IsDeterministic(t *testing.T) {
	keys := []string{"y", "Y", "ctrl+y", "?", "f1", "f10", "d", "D", "1", "2", "10"}
	want := wkSortedKeys(keys...)
	for range 5 {
		shuffled := slices.Clone(keys)
		slices.Reverse(shuffled)
		if got := wkSortedKeys(shuffled...); !slices.Equal(got, want) {
			t.Fatalf("input order changed the result: got %v, want %v", got, want)
		}
	}
}

// TestWhichKeyLeaderCells_AreSorted: the panel's entries must come out sorted,
// not in catalog declaration order, which is what made the old panel look
// arbitrary. With the group headers gone this now covers the WHOLE list rather
// than each section separately.
func TestWhichKeyLeaderCells_AreSorted(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	cells := whichKeyTestModel().whichKeyLeaderCells()
	want := slices.Clone(cells)
	sortWhichKeyCells(want, true)
	if !slices.Equal(cells, want) {
		t.Errorf("the panel list is not sorted:\n got %v\nwant %v", cells, want)
	}
	// Spot-check the property directly rather than only against the helper,
	// scoped to one group's run: within a group, no punctuation-only key may
	// precede an alphanumeric one. This no longer holds ACROSS groups — group
	// clustering means an early group's punctuation (e.g. Actions' "ctrl+l")
	// now legitimately sorts ahead of a later group's letter key (e.g. Views'
	// "P") — see TestSortWhichKeyCells_ClustersByGroupThenKey for that
	// boundary case and TestWhichKeyLeaderCells_GroupsFormContiguousRuns for
	// the clustering property itself.
	var curGroup whichKeyGroup
	seenPunct := ""
	for i, c := range cells {
		if i == 0 || c.group != curGroup {
			curGroup = c.group
			seenPunct = ""
		}
		if wkAlphanumRank(c.key) == 1 && seenPunct == "" {
			seenPunct = c.key
			continue
		}
		if seenPunct != "" && wkAlphanumRank(c.key) == 0 {
			t.Errorf("within group %q, alphanumeric key %q sorts after punctuation key %q", c.group, c.key, seenPunct)
		}
	}
}

// TestSortWhichKeyCells_ClustersByGroupThenKey is the grouping invariant the
// task calls for: each group's entries land as one contiguous run, the runs
// appear in whichKeyGroupOrder order, and the within-group order still
// follows the plain key sort (TestSortWhichKeyCells_PortsNeovimOrdering).
// Input is deliberately shuffled and interleaved across groups — including a
// group whose only entry is punctuation-ranked (Sort, all of ">"/"<"/"="/"-")
// sitting ahead of a later group's alphanumeric keys, the exact boundary case
// TestWhichKeyLeaderCells_AreSorted's old whole-list spot-check would have
// flagged — to prove the group pass, not incoming order, decides adjacency.
func TestSortWhichKeyCells_ClustersByGroupThenKey(t *testing.T) {
	cells := []whichKeyCell{
		{key: "-", desc: "Reset sort", group: wkSort},
		{key: "y", desc: "Copy name", group: wkActions},
		{key: "ctrl+d", desc: "Diff", group: wkSelection},
		{key: "f", desc: "Filter", group: wkFilter},
		{key: ">", desc: "Sort next", group: wkSort},
		{key: "P", desc: "Preview", group: wkViews},
		{key: "ctrl+l", desc: "Logs", group: wkActions},
		{key: "w", desc: "Watch", group: wkSettings},
		{key: "a", desc: "Toggle select", group: wkSelection},
		{key: "T", desc: "Theme", group: wkSettings},
	}
	sortWhichKeyCells(cells, true)

	// Runs must be contiguous and appear in whichKeyGroupOrder order.
	order := whichKeyGroupOrder()
	rankOf := func(g whichKeyGroup) int {
		for i, og := range order {
			if og == g {
				return i
			}
		}
		t.Fatalf("fixture group %q is not declared in whichKeyGroupOrder", g)
		return -1
	}
	seenGroups := []whichKeyGroup{}
	for i, c := range cells {
		if i == 0 || cells[i-1].group != c.group {
			if len(seenGroups) > 0 && seenGroups[len(seenGroups)-1] == c.group {
				t.Fatalf("group %q appears in two separate runs: %v", c.group, cells)
			}
			seenGroups = append(seenGroups, c.group)
		}
	}
	for i := 1; i < len(seenGroups); i++ {
		if rankOf(seenGroups[i-1]) >= rankOf(seenGroups[i]) {
			t.Fatalf("group runs are not in whichKeyGroupOrder order: %v", seenGroups)
		}
	}

	// Within each run, the order matches the plain key sort of that subset.
	byGroup := map[whichKeyGroup][]whichKeyCell{}
	for _, c := range cells {
		byGroup[c.group] = append(byGroup[c.group], c)
	}
	for g, got := range byGroup {
		want := slices.Clone(got)
		sortWhichKeyCells(want, true)
		if !slices.Equal(got, want) {
			t.Errorf("group %q run is not key-sorted:\n got %v\nwant %v", g, got, want)
		}
	}
}

// TestWhichKeyCells_GotoPopupUsesTheSameOrdering: both which-key surfaces share
// one sorter, so the goto popup can't drift back to its old ad-hoc compare.
func TestWhichKeyCells_GotoPopupUsesTheSameOrdering(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	cells := gotoTestModel().whichKeyCells()
	want := slices.Clone(cells)
	sortWhichKeyCells(want, true)
	if !slices.Equal(cells, want) {
		t.Fatalf("goto cells are not in which-key order:\n got %v\nwant %v", cells, want)
	}
	// "\\" (Previous namespace) is punctuation and must land last, after every
	// letter chord.
	if last := cells[len(cells)-1].key; !strings.Contains(last, "\\") {
		t.Fatalf("the punctuation chord must sort last, got %q", last)
	}
}

// TestSortWhichKeyCells_ExplicitOrderOverridesTheKeySort is CHANGE 2's core
// case (USER DECISION): "<" then ">" then "=", with "-" after both — the
// exact pair-splitting complaint a plain ASCII/natural compare produces
// (natural key: "-" < "<" < "=" < ">"). Order 1/2/3 are given directly on the
// cells here rather than via the registry, so this pins the sort's own
// behavior independent of the catalog's specific values (whichkey_registry.go
// covers wiring those into the Sort group).
func TestSortWhichKeyCells_ExplicitOrderOverridesTheKeySort(t *testing.T) {
	cells := []whichKeyCell{
		{key: "=", desc: "Flip sort direction", group: wkSort, order: 3},
		{key: "-", desc: "Reset sort", group: wkSort}, // unset: falls through
		{key: ">", desc: "Sort next column", group: wkSort, order: 2},
		{key: "<", desc: "Sort previous column", group: wkSort, order: 1},
	}
	sortWhichKeyCells(cells, true)
	want := []string{"<", ">", "=", "-"}
	got := make([]string, len(cells))
	for i, c := range cells {
		got[i] = c.key
	}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestSortWhichKeyCells_ExplicitOrderDoesNotLeakAcrossGroups: Order is a
// per-cell field, not a global rank — a group that sets it (Sort) must not
// perturb a group that never opted in. The Actions cells below carry no
// order and must keep sorting by the plain key chain exactly as if the Sort
// group's cells weren't in the same slice at all.
func TestSortWhichKeyCells_ExplicitOrderDoesNotLeakAcrossGroups(t *testing.T) {
	cells := []whichKeyCell{
		{key: "=", desc: "Flip sort direction", group: wkSort, order: 3},
		{key: "y", desc: "Copy name", group: wkActions},
		{key: ">", desc: "Sort next column", group: wkSort, order: 2},
		{key: "D", desc: "Delete", group: wkActions},
		{key: "<", desc: "Sort previous column", group: wkSort, order: 1},
		{key: "d", desc: "Diff", group: wkActions},
	}
	sortWhichKeyCells(cells, true)

	var actionsGot []string
	var sortGot []string
	for _, c := range cells {
		switch c.group {
		case wkActions:
			actionsGot = append(actionsGot, c.key)
		case wkSort:
			sortGot = append(sortGot, c.key)
		}
	}
	// Actions has no explicit order on any entry: plain alphanum/natural/case
	// sort applies, same as TestSortWhichKeyCells_PortsNeovimOrdering — "d"
	// and "D" tie on the natural key and split by case (lower first), "y"
	// sorts after both.
	if want := []string{"d", "D", "y"}; !slices.Equal(actionsGot, want) {
		t.Fatalf("Actions group leaked the Sort group's explicit order: got %v, want %v", actionsGot, want)
	}
	if want := []string{"<", ">", "="}; !slices.Equal(sortGot, want) {
		t.Fatalf("Sort group's explicit order was not applied: got %v, want %v", sortGot, want)
	}
}

// TestWkOrderRank_UnsetFallsThroughToTheLargestRank pins the sentinel
// directly: order 0 (unset) must compare larger than any positive explicit
// order, mirroring neovim's own `order` sorter defaulting unset items to
// 1000 (view.lua's M.fields.order) so they fall through to the natural sort.
func TestWkOrderRank_UnsetFallsThroughToTheLargestRank(t *testing.T) {
	if got := wkOrderRank(0); got <= wkOrderRank(1) {
		t.Fatalf("wkOrderRank(0) = %d must be greater than wkOrderRank(1) = %d", got, wkOrderRank(1))
	}
	if got, want := wkOrderRank(5), 5; got != want {
		t.Fatalf("wkOrderRank(5) = %d, want %d (explicit values pass through unchanged)", got, want)
	}
}

// TestWhichKeyRegistry_SortGroupOrderMatchesTheUserDecision is the end-to-end
// version of TestSortWhichKeyCells_ExplicitOrderOverridesTheKeySort, run
// against the real catalog: SortPrev/SortNext/SortFlip/SortReset must render
// as "<", ">", "=", "-" in that order once wired through whichKeyLeaderCells.
func TestWhichKeyRegistry_SortGroupOrderMatchesTheUserDecision(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()

	cells := whichKeyTestModel().whichKeyLeaderCells()
	var sortKeys []string
	for _, c := range cells {
		if c.group == wkSort {
			sortKeys = append(sortKeys, c.key)
		}
	}
	want := []string{"<", ">", "=", "-"}
	if !slices.Equal(sortKeys, want) {
		t.Fatalf("Sort group order = %v, want %v", sortKeys, want)
	}
}
