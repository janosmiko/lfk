package ui

import "strings"

// Tree-guide segments for the explorer tree views. Three characters per depth
// level (narrower than the Resource Map's four) because the explorer columns
// have limited width and objects nest deep.
const (
	treeGuideBranch = "├─ "
	treeGuideLast   = "└─ "
	treeGuideStem   = "│  "
	treeGuideBlank  = "   "

	// minTreeLabelWidth is the label space a tree row keeps even when its
	// guide prefix eats the whole name column (deeply nested rows run wider
	// instead of rendering an empty label).
	minTreeLabelWidth = 8
)

// TreeGuidePrefixes computes the ASCII-art guide prefix for each row of a
// pre-order flattened tree, given the rows' depths. A row at depth d gets d
// stem/blank segments (one per ancestor, stem while that ancestor has later
// siblings) followed by a branch or last-branch connector. Depths must be a
// valid pre-order sequence (each step down increases depth by exactly 1).
func TreeGuidePrefixes(depths []int) []string {
	n := len(depths)
	prefixes := make([]string, n)

	// A row is the last sibling at its depth unless a later row at the same
	// depth appears before the walk returns to a shallower depth.
	isLast := make([]bool, n)
	for i := range isLast {
		isLast[i] = true
	}
	stack := make([]int, 0, 8)
	for i, d := range depths {
		for len(stack) > 0 && depths[stack[len(stack)-1]] >= d {
			top := stack[len(stack)-1]
			if depths[top] == d {
				isLast[top] = false
			}
			stack = stack[:len(stack)-1]
		}
		stack = append(stack, i)
	}

	var anc []bool // isLast of each ancestor, indexed by depth
	for i, d := range depths {
		if d < len(anc) {
			anc = anc[:d]
		}
		var b strings.Builder
		for _, last := range anc {
			if last {
				b.WriteString(treeGuideBlank)
			} else {
				b.WriteString(treeGuideStem)
			}
		}
		if isLast[i] {
			b.WriteString(treeGuideLast)
		} else {
			b.WriteString(treeGuideBranch)
		}
		prefixes[i] = b.String()
		anc = append(anc, isLast[i])
	}
	return prefixes
}
