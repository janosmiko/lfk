package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/stretchr/testify/assert"
)

func TestRenderBackgroundTasksOverlayEmpty(t *testing.T) {
	t.Parallel()
	got := RenderBackgroundTasksOverlay(nil, 60, 15)
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
	got := RenderBackgroundTasksOverlay(rows, 80, 15)

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
		got := RenderBackgroundTasksOverlay(rows, w, 15)
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
		got := RenderBackgroundTasksOverlay(rows, w, 15)
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
	got := RenderBackgroundTasksOverlay(rows, 80, 15)
	assert.Contains(t, got, "1 task running")
	assert.NotContains(t, got, "1 tasks")
}
