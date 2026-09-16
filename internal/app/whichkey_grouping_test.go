package app

import (
	"slices"
	"testing"
)

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
