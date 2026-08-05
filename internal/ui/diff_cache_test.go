package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// diffCacheFixture is two documents with a foldable unchanged run
// (ComputeDiffFoldRegionsFromLines wants four) and a change at each end.
func diffCacheFixture() (left, right string) {
	var l, r strings.Builder
	l.WriteString("head: left\n")
	r.WriteString("head: right\n")
	for i := range 8 {
		line := "  same-" + string(rune('a'+i)) + "\n"
		l.WriteString(line)
		r.WriteString(line)
	}
	l.WriteString("tail: left\n")
	r.WriteString("tail: right\n")
	return l.String(), r.String()
}

func TestDiffCache_ReusesTheResolvedDiff(t *testing.T) {
	left, right := diffCacheFixture()
	var c DiffCache

	first := c.Resolve(left, right, nil)
	second := c.Resolve(left, right, nil)
	assert.Same(t, first, second, "identical inputs must not recompute the LCS table")

	// Equal-but-distinct strings still hit: the key is the content.
	assert.Same(t, first, c.Resolve(string([]byte(left)), right, nil))
}

func TestDiffCache_InvalidatesOnContentChange(t *testing.T) {
	left, right := diffCacheFixture()
	var c DiffCache

	first := c.Resolve(left, right, nil)
	changed := c.Resolve(left, right+"extra: field\n", nil)
	assert.NotSame(t, first, changed, "a changed side must resolve afresh")
	assert.Greater(t, len(changed.Visible()), len(first.Visible()))
}

// The fold keys write foldState in place (ExpandDiffFoldForLine) rather than
// handing over a new slice, so a cache that kept the caller's slice as its key
// would compare it against itself and serve the pre-fold line list forever.
func TestDiffCache_InvalidatesOnInPlaceFoldMutation(t *testing.T) {
	left, right := diffCacheFixture()
	var c DiffCache

	regions := ComputeDiffFoldRegions(left, right)
	require.NotEmpty(t, regions, "fixture must produce a foldable region")
	foldState := make([]bool, len(regions))

	unfolded := c.Resolve(left, right, foldState)
	foldState[0] = true
	folded := c.Resolve(left, right, foldState)

	assert.NotSame(t, unfolded, folded, "an in-place fold write must invalidate the memo")
	assert.Less(t, len(folded.Visible()), len(unfolded.Visible()), "collapsing a region must hide lines")
}

// A Model that never went through app_init (a hand-built test model, a zero
// value) still has to resolve — just without memoization.
func TestDiffCache_NilReceiverResolves(t *testing.T) {
	left, right := diffCacheFixture()
	var c *DiffCache

	resolved := c.Resolve(left, right, nil)
	require.NotNil(t, resolved)
	assert.Equal(t, ComputeDiffFoldRegions(left, right), resolved.Regions())

	texts := make([]string, 0, 2*len(resolved.Visible()))
	for i := range resolved.Visible() {
		texts = append(texts, resolved.LineText(i, 0, false), resolved.LineText(i, 1, false))
	}
	assert.Contains(t, texts, "head: left")
	assert.Contains(t, texts, "tail: right")
}
