package ui

import "slices"

// ResolvedDiff is one fully resolved diff: the raw line pairing, the foldable
// regions in it, and the visible-line list a given fold state produces. It is
// opaque outside this package because the raw pairing is — callers ask a
// DiffCache for one and read it through the accessors below.
//
// Treat a value as immutable. A cache hand outs the same pointer to every
// caller, so writing through it corrupts the next reader's view.
type ResolvedDiff struct {
	lines   []diffLine
	regions []DiffFoldRegion
	visible []VisibleDiffLine
}

// Regions is the foldable set the resolved fold state was applied to.
func (r *ResolvedDiff) Regions() []DiffFoldRegion { return r.regions }

// Visible is the line list after folds, i.e. the space a cursor indexes into.
func (r *ResolvedDiff) Visible() []VisibleDiffLine { return r.visible }

// LineText is DiffLineTextIn against this diff, without recomputing it.
func (r *ResolvedDiff) LineText(visibleIdx, side int, unified bool) string {
	return DiffLineTextIn(r.lines, r.visible, visibleIdx, side, unified)
}

// DiffCache memoizes the last ResolvedDiff so a caller that resolves the same
// diff every frame pays computeDiff's O(nxm) LCS table once instead of once
// per frame.
//
// Validity is checked against the INPUTS rather than tracked by a generation
// counter the mutation sites would have to remember to bump: foldState is
// written in place (ExpandDiffFoldForLine) and reallocated on region growth,
// so a counter would be one missed write away from serving a stale fold
// layout, while comparing costs one compare per fold region against a table
// with a cell per line PAIR.
//
// Not safe for concurrent use. A cache belongs to one viewer on one goroutine.
type DiffCache struct {
	left, right string
	foldState   []bool
	resolved    *ResolvedDiff
}

// Resolve returns the diff of left and right under foldState, reusing the
// previous result when all three are unchanged.
//
// Nil-safe: a zero-value holder (a Model built by a test, a tab that never
// opened the diff viewer) resolves correctly, just without memoization.
func (c *DiffCache) Resolve(left, right string, foldState []bool) *ResolvedDiff {
	if c != nil && c.resolved != nil && c.left == left && c.right == right && slices.Equal(c.foldState, foldState) {
		return c.resolved
	}
	lines := computeDiff(left, right)
	regions := ComputeDiffFoldRegionsFromLines(lines)
	resolved := &ResolvedDiff{
		lines:   lines,
		regions: regions,
		visible: BuildVisibleDiffLines(lines, regions, foldState),
	}
	if c == nil {
		return resolved
	}
	c.left, c.right = left, right
	// A copy, not the caller's slice: the fold keys mutate foldState in place,
	// which an aliased key could never report as a change.
	c.foldState = append(c.foldState[:0], foldState...)
	c.resolved = resolved
	return resolved
}
