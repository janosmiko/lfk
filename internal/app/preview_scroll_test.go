package app

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
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

// TestPreviewScroll_LongTextDetailsReachBottom is the regression for a
// ConfigMap (or any resource) whose details exceed the pane height — e.g. a
// multi-line `data:` value. The scroll measurement must reflect the true
// content length, not a capped floor, so the last line is reachable.
func TestPreviewScroll_LongTextDetailsReachBottom(t *testing.T) {
	var body strings.Builder
	for i := range 300 {
		fmt.Fprintf(&body, "    config line %d\n", i)
	}
	cm := model.Item{
		Kind: "ConfigMap", Name: "varnish", Namespace: "ns", Age: "1d",
		Columns: []model.KeyValue{
			{Key: "Data", Value: "1 keys"},
			{Key: "data:default.vcl.tmpl", Value: body.String()},
		},
	}
	m := Model{
		nav:         model.NavigationState{Level: model.LevelResources, ResourceType: model.ResourceTypeEntry{Kind: "ConfigMap"}},
		middleItems: []model.Item{cm},
		width:       120, height: 50,
	}
	m.cursors[model.LevelResources] = 0

	innerW := 41
	contentHeight := max(m.height-4, 3)
	measured := m.measureScrollableLines(innerW, contentHeight)
	trueLines := strings.Count(m.renderRightColumnContent(innerW, 1<<20), "\n") + 1

	if measured != trueLines {
		t.Errorf("measured line count must equal true content: measured=%d true=%d", measured, trueLines)
	}
	if measured <= 200 {
		t.Errorf("long multi-line data value should measure past the old 200 cap, got %d", measured)
	}
}

// TestPreviewScroll_ListWindowed verifies the windowed list render is
// byte-equivalent to the old line-slice path (it is a pure performance change):
// previewScroll is a display-line offset where line 0 is the header and line i
// is item i-1, so a scrolled view drops the header and starts at item
// previewScroll-1 — never shifted by a row.
func TestPreviewScroll_ListWindowed(t *testing.T) {
	m := resourceTypePreview(scrollPodItems(1000))

	// At the top: header visible, first data row is pod-0.
	m.previewScroll = 0
	out0 := ansi.Strip(m.renderRightColumn(80, 46))
	if !strings.Contains(out0, "NAME") {
		t.Errorf("header must be visible at the top of the list")
	}
	if top := topPodIndex(m); top != 0 {
		t.Errorf("top item at scroll 0 = pod-%d, want pod-0", top)
	}

	// Scrolled: header scrolls off (like every other preview), and the top data
	// row is item previewScroll-1 (line previewScroll), matching the line-slice.
	m.previewScroll = 500
	out500 := ansi.Strip(m.renderRightColumn(80, 46))
	if strings.Contains(out500, "NAME") {
		t.Errorf("header should scroll off the top once scrolled")
	}
	if top := topPodIndex(m); top != 499 {
		t.Errorf("top item at scroll 500 = pod-%d, want pod-499", top)
	}
	if strings.Contains(out500, "pod-0 ") {
		t.Errorf("items far above the window must not be rendered")
	}

	// Max offset still shows the last item.
	m.previewScroll = 999
	if !strings.Contains(ansi.Strip(m.renderRightColumn(80, 46)), "pod-999") {
		t.Errorf("last item must remain reachable at the maximum offset")
	}
}

// TestPreviewScroll_ListWindowedEquivalence pins the windowed render to the
// reference line-slice output across scroll positions, proving the optimization
// changes nothing the user sees.
func TestPreviewScroll_ListWindowedEquivalence(t *testing.T) {
	const width = 80
	m := resourceTypePreview(scrollPodItems(1000))

	// Reference: full content rendered from the top, then line-sliced (the old
	// behaviour). ActiveRightScroll<0 disables windowing for this measurement.
	ui.ActiveRightScroll = -1
	full := strings.Split(ansi.Strip(m.renderRightColumnContent(width, 1<<20)), "\n")

	// Only compare the upper content rows; the bottom of the pane holds the pinned
	// summary-band footer, which is not part of the scrollable content. 40 rows is
	// safely within the content region for a height of 46.
	const contentRows = 40
	for _, p := range []int{0, 1, 137, 500, 953} {
		m.previewScroll = p
		got := strings.Split(ansi.Strip(m.renderRightColumn(width, 46)), "\n")
		for i := 0; i < contentRows && i < len(got) && p+i < len(full); i++ {
			if got[i] != full[p+i] {
				t.Fatalf("scroll=%d row=%d: windowed %q != line-slice %q", p, i, got[i], full[p+i])
			}
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
