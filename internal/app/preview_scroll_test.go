package app

import (
	"fmt"
	"regexp"
	"strconv"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
)

func scrollPodItems(n int) []model.Item {
	items := make([]model.Item, n)
	for i := range items {
		items[i] = model.Item{
			Kind: "Pod", Name: fmt.Sprintf("pod-%d", i),
			Namespace: "default", Status: "Running", Ready: "1/1",
		}
	}
	return items
}

func resourceTypePreview(items []model.Item) Model {
	return Model{
		nav:         model.NavigationState{Level: model.LevelResourceTypes},
		middleItems: []model.Item{{Kind: "Pod", Name: "Pods"}},
		rightItems:  items,
		width:       120,
		height:      50,
	}
}

var rePodIdx = regexp.MustCompile(`pod-(\d+)`)

// maxPodIndexVisible returns the highest pod index present in the rendered
// right column, or -1 if none.
func maxPodIndexVisible(m Model) int {
	out := ansi.Strip(m.renderRightColumn(80, 46))
	maxIdx := -1
	for _, mm := range rePodIdx.FindAllStringSubmatch(out, -1) {
		if v, err := strconv.Atoi(mm[1]); err == nil && v > maxIdx {
			maxIdx = v
		}
	}
	return maxIdx
}

func topPodIndex(m Model) int {
	out := ansi.Strip(m.renderRightColumn(80, 46))
	if mm := rePodIdx.FindStringSubmatch(out); mm != nil {
		v, _ := strconv.Atoi(mm[1])
		return v
	}
	return -1
}

// TestPreviewScroll_ReachesBottomForLongList is the regression for the right-pane
// scroll freezing partway down a long list: previewScroll used to cap at ~150
// regardless of list length because the content measurement was capped, so the
// bottom of a few-hundred-item list was unreachable.
func TestPreviewScroll_ReachesBottomForLongList(t *testing.T) {
	for _, n := range []int{120, 300, 1000} {
		m := resourceTypePreview(scrollPodItems(n))
		for range n + 50 {
			mdl, _, _ := m.handleExplorerActionKeyPreviewDown()
			m = mdl.(Model)
		}
		if got := maxPodIndexVisible(m); got != n-1 {
			t.Errorf("n=%d: last item (pod-%d) never reached; highest visible = pod-%d", n, n-1, got)
		}
	}
}

// TestPreviewScroll_NoDeadBandAtBottom verifies that one scroll-up from the
// bottom moves the viewport by exactly one row — previously previewScroll could
// overshoot into a dead band where scroll-up did nothing until it unwound below
// the real maximum.
func TestPreviewScroll_NoDeadBandAtBottom(t *testing.T) {
	m := resourceTypePreview(scrollPodItems(300))
	for range 400 {
		mdl, _, _ := m.handleExplorerActionKeyPreviewDown()
		m = mdl.(Model)
	}
	topAtBottom := topPodIndex(m)

	mdl, _, _ := m.handleExplorerActionKeyPreviewUp()
	m = mdl.(Model)
	topAfterUp := topPodIndex(m)

	if topAfterUp != topAtBottom-1 {
		t.Errorf("scroll-up at bottom did not move viewport by one: top %d -> %d (want %d)",
			topAtBottom, topAfterUp, topAtBottom-1)
	}
}

// TestPreviewScroll_MeasureMemoized verifies the line-count measurement is
// cached across scroll keystrokes (it only recomputes when the content key
// changes), which is what keeps scrolling a large list cheap.
func TestPreviewScroll_MeasureMemoized(t *testing.T) {
	m := resourceTypePreview(scrollPodItems(500))

	innerW, scrollableH := 100, 40
	first := m.measureScrollableLines(innerW, scrollableH)
	if first <= 500 {
		t.Fatalf("expected measured lines > item count (got %d)", first)
	}
	cachedKey := m.previewMeasureKey

	// Same inputs -> cache hit, identical result, key unchanged.
	if got := m.measureScrollableLines(innerW, scrollableH); got != first {
		t.Errorf("memoized measure changed without input change: %d != %d", got, first)
	}
	if m.previewMeasureKey != cachedKey {
		t.Errorf("measure key changed on a cache hit")
	}

	// Changing the list length invalidates the cache and recomputes.
	m.rightItems = scrollPodItems(900)
	grown := m.measureScrollableLines(innerW, scrollableH)
	if grown <= first {
		t.Errorf("measure did not grow after list grew: %d <= %d", grown, first)
	}

	// Shrinking the list must also recompute downward (not return the stale
	// larger value).
	m.rightItems = scrollPodItems(10)
	if got := m.measureScrollableLines(innerW, scrollableH); got >= grown {
		t.Errorf("measure did not shrink after list shrank: %d >= %d", got, grown)
	}
}

// TestPreviewScroll_CursorChangeInvalidatesMeasure guards against a stale
// measurement after a cursor-driven preview change. The memo key is coarse
// (e.g. it does not capture the split details-summary length), so the
// content-reset paths must clear the cache or a new selection could be clamped
// against the previous preview's line count.
func TestPreviewScroll_CursorChangeInvalidatesMeasure(t *testing.T) {
	m := resourceTypePreview(scrollPodItems(500))
	m.measureScrollableLines(100, 40)
	if m.previewMeasureLines == 0 {
		t.Fatal("precondition: expected a cached measurement")
	}

	m.invalidatePreviewForCursorChange()

	if m.previewMeasureLines != 0 {
		t.Errorf("cursor-change invalidation must clear the cached measurement, got %d", m.previewMeasureLines)
	}
}
