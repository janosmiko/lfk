package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// wkDiffBenchModel is a diff viewer holding two manifests of a size the app
// really opens (a ~400-line rendered Deployment against its live copy), with
// enough unchanged runs to make folds real. computeDiff is O(nxm), so the cost
// this measures grows quadratically with the document — a 40-line fixture
// would hide the whole finding.
func wkDiffBenchModel() Model {
	var left, right strings.Builder
	for i := range 400 {
		fmt.Fprintf(&left, "  field-%03d: value-%03d\n", i, i)
		if i%50 == 0 {
			fmt.Fprintf(&right, "  field-%03d: changed-%03d\n", i, i)
			continue
		}
		fmt.Fprintf(&right, "  field-%03d: value-%03d\n", i, i)
	}
	m := whichKeyTestModel()
	m.mode = modeDiff
	m.width, m.height = 120, 40
	m.diffView.left, m.diffView.right = left.String(), right.String()
	m.diffView.leftName, m.diffView.rightName = "live", "desired"
	return m
}

// TestWhichKeyDiff_MemoFollowsAFoldToggle drives the real fold key and then
// re-resolves the context. toggleAllDiffFolds writes foldState IN PLACE, so a
// memo keyed on the caller's slice would compare it against itself and keep
// serving the pre-fold visible-line list — the panel would then gate its
// entries on lines the viewer no longer shows.
func TestWhichKeyDiff_MemoFollowsAFoldToggle(t *testing.T) {
	restoreWhichKeyGlobals(t)
	kb := ui.DefaultKeybindings()
	ui.ActiveKeybindings = kb

	m := wkDiffBenchModel()
	regions := ui.ComputeDiffFoldRegions(m.diffView.left, m.diffView.right)
	if len(regions) == 0 {
		t.Fatal("precondition: the fixture must produce foldable regions")
	}
	m.ensureDiffFoldState(regions)

	before := len(newWKDiffCtx(&m).vis)

	out, _ := m.handleDiffKey(keyMsg(kb.ToggleFoldAll))
	folded := out.(Model)
	after := len(newWKDiffCtx(&folded).vis)

	if after >= before {
		t.Errorf("collapsing every region must shrink the visible list; %d -> %d", before, after)
	}
}

// BenchmarkWhichKeyDiffAvailability measures one availability pass over the
// diff catalog — what primeWhichKeyCells runs on every frame the panel is on
// screen. Counted separately: the panel re-renders per keystroke, and the LCS
// table behind it does not depend on anything a keystroke changes.
func BenchmarkWhichKeyDiffAvailability(b *testing.B) {
	prev := ui.ActiveKeybindings
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	b.Cleanup(func() { ui.ActiveKeybindings = prev })

	m := wkDiffBenchModel()
	b.ReportAllocs()
	for b.Loop() {
		_ = m.availableWhichKeyActions()
	}
}
