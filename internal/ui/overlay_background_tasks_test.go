package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderBackgroundTasksOverlayEmpty(t *testing.T) {
	t.Parallel()
	got := RenderBackgroundTasksOverlay(nil, ModeRunning, 0, 60, 15)
	assert.Contains(t, got, "Background Tasks")
	assert.Contains(t, got, "No background tasks running")
}

func TestRenderBackgroundTasksOverlayWithRows(t *testing.T) {
	t.Parallel()
	now := time.Now()
	rows := []BackgroundTaskRow{
		{Kind: "ResourceList", Name: "List Pods", Target: "default", StartedAt: now.Add(-3 * time.Second)},
		{Kind: "YAMLFetch", Name: "Get YAML", Target: "default/web-7d8c", StartedAt: now.Add(-1200 * time.Millisecond)},
		{Kind: "Metrics", Name: "Pod metrics", Target: "default", StartedAt: now.Add(-8700 * time.Millisecond)},
	}
	got := RenderBackgroundTasksOverlay(rows, ModeRunning, 0, 80, 15)

	// Header.
	assert.Contains(t, got, "Background Tasks")
	// Column headers.
	assert.Contains(t, got, "KIND")
	assert.Contains(t, got, "NAME")
	assert.Contains(t, got, "TARGET")
	assert.Contains(t, got, "ELAPSED")
	// Row data.
	assert.Contains(t, got, "ResourceList")
	assert.Contains(t, got, "List Pods")
	assert.Contains(t, got, "default")
	assert.Contains(t, got, "Pod metrics")
	// Footer count.
	assert.Contains(t, got, "3 tasks running")
}

func TestFormatElapsedBGT(t *testing.T) {
	t.Parallel()
	cases := []struct {
		d    time.Duration
		want string
	}{
		{500 * time.Millisecond, "0.5s"},
		{1200 * time.Millisecond, "1.2s"},
		{3500 * time.Millisecond, "3.5s"},
		{9900 * time.Millisecond, "9.9s"},
		{12 * time.Second, "12s"},
		{59 * time.Second, "59s"},
		{60 * time.Second, "1m 0s"},
		{90 * time.Second, "1m 30s"},
		{125 * time.Second, "2m 5s"},
		{-500 * time.Millisecond, "-0.5s"}, // clock skew edge case
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, formatElapsedBGT(tc.d), "duration %s", tc.d)
	}
}

func TestRenderBackgroundTasksOverlayFitsInWidth(t *testing.T) {
	t.Parallel()
	rows := []BackgroundTaskRow{
		{Kind: "ResourceList", Name: "List Pods", Target: "default", StartedAt: time.Now()},
	}
	for _, w := range []int{60, 80, 100, 120} {
		got := RenderBackgroundTasksOverlay(rows, ModeRunning, 0, w, 15)
		actualWidth := lipgloss.Width(got)
		assert.LessOrEqual(t, actualWidth, w,
			"overlay must not exceed configured width %d (got %d)", w, actualWidth)
	}
}

func TestRenderBackgroundTasksOverlayFitsInWidthWideRows(t *testing.T) {
	t.Parallel()
	// Realistic worst-case: long CRD kind, long pod name, long target.
	rows := []BackgroundTaskRow{
		{
			Kind:      "VeryLongCustomResourceKind",
			Name:      "very-long-operation-name-goes-here",
			Target:    "very-long-namespace/with-slashed/path",
			StartedAt: time.Now().Add(-5 * time.Second),
		},
	}
	for _, w := range []int{50, 60, 80, 100} {
		got := RenderBackgroundTasksOverlay(rows, ModeRunning, 0, w, 15)
		// The overlay content targets width-6 (the caller's OverlayStyle
		// wraps it in a border + padding adding 6 cells of horizontal
		// overhead), so actualWidth must be strictly less than w-6 plus
		// some styling slack. Asserting <=w is loose but sufficient to
		// catch "content is bigger than the overlay box" regressions.
		actualWidth := lipgloss.Width(got)
		assert.LessOrEqual(t, actualWidth, w,
			"overlay with wide data must still fit in width %d (got %d)", w, actualWidth)
		// Lines must not wrap. The inner content for 1 data row is
		// "Background Tasks\n\nHEADER\nROW\n\nN tasks running" — 5 newlines.
		// Any extra newline means a row wrapped onto a second line.
		lines := strings.Count(got, "\n")
		assert.LessOrEqual(t, lines, 5,
			"overlay with 1 data row should not wrap; got %d lines", lines)
	}
}

func TestRenderBackgroundTasksOverlaySingleTaskFooter(t *testing.T) {
	t.Parallel()
	// Singular noun when count == 1.
	rows := []BackgroundTaskRow{
		{Kind: "ResourceList", Name: "List Pods", Target: "default", StartedAt: time.Now().Add(-2 * time.Second)},
	}
	got := RenderBackgroundTasksOverlay(rows, ModeRunning, 0, 80, 15)
	assert.Contains(t, got, "1 task running")
	assert.NotContains(t, got, "1 tasks")
}

// --- Completed mode ---

func TestRenderBackgroundTasksOverlayCompletedEmpty(t *testing.T) {
	t.Parallel()
	got := RenderBackgroundTasksOverlay(nil, ModeCompleted, 0, 60, 15)
	assert.Contains(t, got, "Completed Tasks",
		"completed mode must show the Completed Tasks title")
	assert.Contains(t, got, "No completed tasks yet",
		"completed mode must show the no-history empty state")
}

func TestRenderBackgroundTasksOverlayCompletedRendersDuration(t *testing.T) {
	t.Parallel()
	// In Completed mode the final column is DURATION and its value is
	// read from row.Duration directly — NOT computed from StartedAt.
	// This matches how the registry exposes CompletedTask.Duration().
	rows := []BackgroundTaskRow{
		{
			Kind:     "ResourceList",
			Name:     "List Pods",
			Target:   "prod / default",
			Duration: 1200 * time.Millisecond,
		},
		{
			Kind:     "YAMLFetch",
			Name:     "Preview YAML",
			Target:   "prod / default",
			Duration: 3 * time.Second,
		},
	}
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 0, 80, 15)

	assert.Contains(t, got, "Completed Tasks")
	assert.Contains(t, got, "DURATION",
		"completed mode must label the right column as DURATION")
	assert.NotContains(t, got, "ELAPSED",
		"completed mode must NOT show the running-mode ELAPSED column")
	assert.Contains(t, got, "1.2s")
	assert.Contains(t, got, "3.0s")
	assert.Contains(t, got, "List Pods")
	assert.Contains(t, got, "Preview YAML")
}

func TestRenderBackgroundTasksOverlayCompletedFooter(t *testing.T) {
	t.Parallel()
	rows := []BackgroundTaskRow{
		{Kind: "ResourceList", Name: "a", Target: "t", Duration: 1 * time.Second},
		{Kind: "ResourceList", Name: "b", Target: "t", Duration: 2 * time.Second},
		{Kind: "ResourceList", Name: "c", Target: "t", Duration: 3 * time.Second},
	}
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 0, 80, 15)
	// Footer says "3 tasks completed", not "3 tasks running".
	assert.Contains(t, got, "3 tasks completed")
	assert.NotContains(t, got, "running")
}

func TestRenderBackgroundTasksOverlayCompletedSingleFooter(t *testing.T) {
	t.Parallel()
	rows := []BackgroundTaskRow{
		{Kind: "ResourceList", Name: "only", Target: "t", Duration: 500 * time.Millisecond},
	}
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 0, 80, 15)
	assert.Contains(t, got, "1 task completed")
	assert.NotContains(t, got, "1 tasks")
}

// --- Scrolling ---

// manyRows builds n completed rows for scroll tests.
func manyRows(n int) []BackgroundTaskRow {
	rows := make([]BackgroundTaskRow, n)
	for i := range rows {
		rows[i] = BackgroundTaskRow{
			Kind:     "ResourceList",
			Name:     fmt.Sprintf("row-%02d", i),
			Target:   "t",
			Duration: time.Second,
		}
	}
	return rows
}

// TestRenderBackgroundTasksOverlayClipsToHeight pins that the renderer
// output never exceeds the number of lines we can fit into the passed
// height. Previously the overlay grew vertically as new tasks arrived
// because lipgloss's outer Height() did not clip the inner content.
func TestRenderBackgroundTasksOverlayClipsToHeight(t *testing.T) {
	t.Parallel()
	rows := manyRows(50)
	// height = 15 → the inner content must stay short enough that the
	// caller's OverlayStyle wrapping cannot spill past the 15-row box.
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 0, 80, 15)
	// Each "\n" separates a line; the final line has no trailing
	// newline. Line count = count("\n") + 1.
	lines := strings.Count(got, "\n") + 1
	assert.LessOrEqual(t, lines, 15,
		"renderer must not emit more than `height` lines; got %d", lines)
}

// TestRenderBackgroundTasksOverlayScrollOffsetSlices pins that the
// scroll parameter slides the visible window through the row list.
// scroll=10 with a 50-item list must hide the first 10 rows and show
// row-10 as the first visible entry.
func TestRenderBackgroundTasksOverlayScrollOffsetSlices(t *testing.T) {
	t.Parallel()
	rows := manyRows(50)
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 10, 80, 15)

	// row-10 must be present (start of the window) and row-00 must NOT be
	// (scrolled past).
	assert.Contains(t, got, "row-10", "scrolled window must include row-10")
	assert.NotContains(t, got, "row-00",
		"rows above the scroll offset must not appear")
}

// TestRenderBackgroundTasksOverlayFooterShowsScrollPosition pins that
// when the history is larger than the visible window, the footer
// carries a "(X-Y)" indicator so users know where they are in the list.
func TestRenderBackgroundTasksOverlayFooterShowsScrollPosition(t *testing.T) {
	t.Parallel()
	rows := manyRows(50)
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 0, 80, 15)
	// Footer must include a position indicator when there's overflow.
	assert.Regexp(t, `\(1-\d+\)`, got,
		"footer must show scroll position when rows exceed the window")
}

// TestRenderBackgroundTasksOverlayFooterOmitsPositionWhenAllFit pins
// the inverse: if every row fits in the window, no position indicator
// is shown — the indicator is noise when you can see everything.
func TestRenderBackgroundTasksOverlayFooterOmitsPositionWhenAllFit(t *testing.T) {
	t.Parallel()
	rows := manyRows(3)
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 0, 80, 30)
	assert.NotRegexp(t, `\(\d+-\d+\)`, got,
		"no position indicator when everything fits")
}

// TestRenderBackgroundTasksOverlayScrollClampsToValidRange pins that
// passing a scroll value beyond the end clamps to the last valid
// window start, so the user can't scroll past the last row.
func TestRenderBackgroundTasksOverlayScrollClampsToValidRange(t *testing.T) {
	t.Parallel()
	rows := manyRows(50)
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, 9999, 80, 15)
	// The last row (row-49) must be visible; anything clamped should
	// land at the tail of the list.
	assert.Contains(t, got, "row-49",
		"scrolling past the end must clamp to show the last row")
}

// TestRenderBackgroundTasksOverlayNegativeScrollClampsToZero pins the
// negative clamp — callers that bump scroll-- blindly should not
// produce weird slicing.
func TestRenderBackgroundTasksOverlayNegativeScrollClampsToZero(t *testing.T) {
	t.Parallel()
	rows := manyRows(50)
	got := RenderBackgroundTasksOverlay(rows, ModeCompleted, -5, 80, 15)
	assert.Contains(t, got, "row-00",
		"negative scroll must clamp to the top of the list")
}
