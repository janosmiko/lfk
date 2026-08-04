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
	sortWhichKeyCells(cells)
	out := make([]string, len(cells))
	for i, c := range cells {
		out[i] = c.key
	}
	return out
}

// TestSortWhichKeyCells_PortsNeovimOrdering covers each sorter in the ported
// chain (view.lua:20-51 with config.lua's `sort`, plus the appended natural and
// case sorters) in isolation and then together.
func TestSortWhichKeyCells_PortsNeovimOrdering(t *testing.T) {
	tests := []struct {
		name string
		in   []string
		want []string
	}{
		{
			"alphanum before everything else",
			[]string{"/", "y", "ctrl+a", "!", "b"},
			[]string{"b", "y", "ctrl+a", "!", "/"},
		},
		{
			"modifier chords before other punctuation",
			[]string{"@", "ctrl+y", "\\", "alt+b"},
			[]string{"alt+b", "ctrl+y", "@", "\\"},
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
			[]string{"a", "f1", "y", "Y", "ctrl+a", "ctrl+space", ".", "?"},
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
	sortWhichKeyCells(want)
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
	sortWhichKeyCells(cells)

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
		sortWhichKeyCells(want)
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
	sortWhichKeyCells(want)
	if !slices.Equal(cells, want) {
		t.Fatalf("goto cells are not in which-key order:\n got %v\nwant %v", cells, want)
	}
	// "\\" (Previous namespace) is punctuation and must land last, after every
	// letter chord.
	if last := cells[len(cells)-1].key; !strings.Contains(last, "\\") {
		t.Fatalf("the punctuation chord must sort last, got %q", last)
	}
}
