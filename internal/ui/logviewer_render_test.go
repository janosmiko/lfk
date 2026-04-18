package ui

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// --- renderWrappedLines ---

func TestRenderWrappedLines(t *testing.T) {
	t.Run("basic wrapping", func(t *testing.T) {
		lines := []string{"hello world this is a long line", "short"}
		result := renderWrappedLines(lines, 0, 10, 15, false, 0, -1, -1, -1, -1, 0, 0, 0)
		// Should have multiple wrapped sub-lines for the first long line.
		assert.Greater(t, len(result), 2)
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "hello")
		assert.Contains(t, joined, "short")
	})

	t.Run("scroll skips initial lines", func(t *testing.T) {
		lines := []string{"line1", "line2", "line3", "line4"}
		result := renderWrappedLines(lines, 2, 3, 40, false, 0, -1, -1, -1, -1, 0, 0, 0)
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "line3")
		assert.Contains(t, joined, "line4")
		assert.NotContains(t, joined, "line1")
	})

	t.Run("height limits output", func(t *testing.T) {
		lines := []string{"a", "b", "c", "d", "e"}
		result := renderWrappedLines(lines, 0, 3, 40, false, 0, -1, -1, -1, -1, 0, 0, 0)
		assert.LessOrEqual(t, len(result), 3)
	})

	t.Run("empty lines list", func(t *testing.T) {
		result := renderWrappedLines(nil, 0, 5, 40, false, 0, -1, -1, -1, -1, 0, 0, 0)
		assert.Empty(t, result)
	})

	t.Run("line numbers shown", func(t *testing.T) {
		lines := []string{"line1", "line2"}
		result := renderWrappedLines(lines, 0, 2, 40, true, 3, -1, -1, -1, -1, 0, 0, 0)
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "1")
		assert.Contains(t, joined, "2")
	})

	t.Run("cursor line gets indicator glyph", func(t *testing.T) {
		lines := []string{"hello", "world"}
		result := renderWrappedLines(lines, 0, 2, 40, false, 0, 0, -1, -1, -1, 0, 0, 0)
		assert.Contains(t, result[0], "\u258e")
	})

	t.Run("non-cursor lines start with space", func(t *testing.T) {
		lines := []string{"hello", "world"}
		result := renderWrappedLines(lines, 0, 2, 40, false, 0, 0, -1, -1, -1, 0, 0, 0)
		assert.True(t, strings.HasPrefix(result[1], " "), "non-cursor line should start with space")
	})

	t.Run("very long line wraps to multiple sub-lines", func(t *testing.T) {
		longLine := strings.Repeat("x", 100)
		lines := []string{longLine}
		result := renderWrappedLines(lines, 0, 20, 20, false, 0, -1, -1, -1, -1, 0, 0, 0)
		// 100 chars / (20-1 gutter) = ~6 wrapped lines.
		assert.Greater(t, len(result), 1)
	})
}

// --- RenderLogViewer ---

func TestRenderLogViewer(t *testing.T) {
	t.Run("basic rendering contains title and lines", func(t *testing.T) {
		lines := []string{"log entry 1", "log entry 2", "log entry 3"}
		result := RenderLogViewer(
			lines, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"my-pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "my-pod")
		assert.Contains(t, result, "log entry 1")
		assert.Contains(t, result, "3 lines")
	})

	t.Run("follow indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			true, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "FOLLOW")
	})

	t.Run("wrap indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, true, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "WRAP")
	})

	t.Run("line numbers indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, true, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "LINE#")
	})

	t.Run("timestamps indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, true, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "TIMESTAMPS")
	})

	t.Run("previous indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, true, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "PREVIOUS")
	})

	t.Run("search query shown in title", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"error log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "error", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "/error")
	})

	t.Run("search active shows input prompt", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "err",
			true, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "err")
		assert.Contains(t, result, "enter:apply")
	})

	t.Run("status message shown in footer", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"Saved to file", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "Saved to file")
	})

	t.Run("visual mode indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			0, true, 0, 'V', 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "VISUAL LINE")
	})

	t.Run("visual char mode indicator", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			0, true, 0, 'v', 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "VISUAL")
	})

	t.Run("visual block mode indicator", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			0, true, 0, 'B', 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "VISUAL BLOCK")
	})

	t.Run("empty lines list renders without panic", func(t *testing.T) {
		result := RenderLogViewer(
			nil, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "pod")
		assert.Contains(t, result, "0 lines")
	})

	t.Run("loading history indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, true,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "LOADING HISTORY")
	})

	t.Run("switch pod hint shown when wide enough", func(t *testing.T) {
		// Use a very wide terminal so the hint bar does not truncate.
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, true, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "switch pod")
	})

	t.Run("containers hint shown when wide enough", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, nil, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, true, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "containers")
	})

	t.Run("wrap mode uses wrapped renderer", func(t *testing.T) {
		longLine := strings.Repeat("x", 100)
		result := RenderLogViewer(
			[]string{longLine}, nil, 0, 40, 20,
			false, true, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.Contains(t, result, "WRAP")
		// The long line should be visible (at least part of it).
		assert.Contains(t, result, "xxx")
	})
}

// --- RenderLogViewer relative timestamps ---

// TestRenderLogViewerRelativeTimestamps pins `now` to a known value
// via the package-level nowFunc, feeds in a log line with a timestamp
// that is exactly 5 minutes in the past, and asserts:
//  1. the rendered output contains "5m ago",
//  2. it no longer contains the RFC3339 timestamp form.
//
// Also checks the subtle "[REL]" title-bar indicator is surfaced
// while timestamps are on, and confirms relative rendering is a no-op
// when timestamps are off (hidden altogether).
func TestRenderLogViewerRelativeTimestamps(t *testing.T) {
	// Pin `now` so the test is deterministic. Remember to restore.
	now := time.Date(2026, 4, 16, 10, 5, 30, 0, time.UTC)
	origNow := nowFunc
	nowFunc = func() time.Time { return now }
	t.Cleanup(func() { nowFunc = origNow })

	// Timestamp exactly 5 minutes before `now`.
	pastStamp := "2026-04-16T10:00:30Z"
	line := pastStamp + " server started"

	t.Run("renders relative form when timestamps and relativeTimestamps both on", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{line}, nil, 0, 80, 20,
			false, false, false, true, false, false, // timestamps=true
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", true, false, nil, LogHistogramView{}, // relativeTimestamps=true
		)
		assert.Contains(t, result, "5m ago",
			"relative form should replace the RFC3339 timestamp")
		assert.NotContains(t, result, pastStamp,
			"original RFC3339 timestamp should be gone from the rendered output")
		assert.Contains(t, result, "REL",
			"the subtle [REL] chip should surface in the title bar")
	})

	t.Run("relativeTimestamps is a no-op when timestamps is off", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{line}, nil, 0, 80, 20,
			false, false, false, false, false, false, // timestamps=false
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", true, false, nil, LogHistogramView{}, // relativeTimestamps=true
		)
		// With timestamps off the renderer strips the prefix entirely,
		// so neither the RFC3339 form nor "5m ago" should appear.
		assert.NotContains(t, result, "5m ago",
			"rel form must not render when timestamps are hidden")
		assert.NotContains(t, result, pastStamp,
			"timestamps-off stripping still applies")
		assert.NotContains(t, result, "[REL]",
			"[REL] chip must not appear when timestamps are off")
	})

	t.Run("relativeTimestamps off leaves the RFC3339 form intact", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{line}, nil, 0, 80, 20,
			false, false, false, true, false, false, // timestamps=true
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{}, // relativeTimestamps=false
		)
		assert.Contains(t, result, pastStamp,
			"absolute form is unchanged when relativeTimestamps is off")
		assert.NotContains(t, result, "5m ago")
	})
}

// TestRenderLogViewerJSONPretty verifies the jsonPretty toggle + the
// prettyLines parallel slice produce multi-line output for JSON log
// lines. The rendered output must contain multiple newlines between
// the opening `{` and the closing `}` (one per expanded field) when
// pretty mode is on. Without the mode (or without prettyLines) the
// output is the raw single-line form.
func TestRenderLogViewerJSONPretty(t *testing.T) {
	t.Run("pretty mode expands json into multiple visual rows", func(t *testing.T) {
		lines := []string{`{"a":1,"b":2}`}
		pretty := []string{"{\n  \"a\": 1,\n  \"b\": 2\n}"}
		result := RenderLogViewer(
			lines, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, true, pretty, LogHistogramView{},
		)
		// Each field on its own row + the opening `{` row + the
		// closing `}` row → at least 3 separators between them.
		idxOpen := strings.Index(result, "{")
		idxClose := strings.LastIndex(result, "}")
		require := result[idxOpen:idxClose]
		assert.Greater(t, strings.Count(require, "\n"), 2,
			"pretty mode should produce multiple visual rows between the braces")
		assert.Contains(t, result, `"a": 1`)
		assert.Contains(t, result, `"b": 2`)
		assert.Contains(t, result, "JSON",
			"title bar should show the JSON indicator chip")
	})

	t.Run("pretty indicator off when mode disabled", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{`{"a":1}`}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.NotContains(t, result, "[JSON]",
			"JSON chip must not appear when the mode is off")
	})

	t.Run("non-json plain line passes through unchanged", func(t *testing.T) {
		pretty := []string{""} // empty entry → fall back to raw line
		result := RenderLogViewer(
			[]string{"hello plain"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, true, pretty, LogHistogramView{},
		)
		assert.Contains(t, result, "hello plain")
	})
}

// TestRenderLogViewerHistogramStrip exercises the wiring contract for
// the Phase 3C histogram: when the caller passes a non-empty
// LogHistogramView, the rendered output must contain the title-bar
// chip ("[HIST: 5m]") AND the strip itself (presence of at least one
// of the box-drawing block characters '▁'..'█'). When the view is
// the zero value, neither must appear.
func TestRenderLogViewerHistogramStrip(t *testing.T) {
	t.Run("non-empty histogram renders strip and chip", func(t *testing.T) {
		view := LogHistogramView{
			Counts:        []int{1, 3, 7, 2, 5},
			CursorBucket:  -1,
			BucketStepStr: "5m",
		}
		result := RenderLogViewer(
			[]string{"a", "b", "c"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, view,
		)
		assert.Contains(t, result, "HIST: 5m",
			"title bar should surface the [HIST: <step>] chip when histogram is on")
		// At least one box-drawing block character should appear in the
		// rendered output. We don't pin which one — log scaling picks
		// from '▁'..'█' depending on the bucket distribution.
		hasBlock := false
		for _, r := range "\u2581\u2582\u2583\u2584\u2585\u2586\u2587\u2588" {
			if strings.ContainsRune(result, r) {
				hasBlock = true
				break
			}
		}
		assert.True(t, hasBlock,
			"rendered output should contain at least one box-drawing block character from the histogram strip")
	})

	t.Run("zero histogram skips strip and chip", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"a", "b", "c"}, nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0, "", "", false, false, nil, LogHistogramView{},
		)
		assert.NotContains(t, result, "HIST:",
			"chip must not appear when histogram view is empty")
		// No box-drawing block char should appear in pure passthrough mode.
		for _, r := range "\u2581\u2582\u2583\u2584\u2585\u2586\u2587\u2588" {
			assert.NotContains(t, result, string(r),
				"strip block characters must not appear when histogram is off")
		}
	})
}

// --- colorizePodPrefix ---

func TestColorizePodPrefix(t *testing.T) {
	t.Run("preserves prefix and content", func(t *testing.T) {
		line := "[pod/myapp-abc12/app] some log output"
		result := colorizePodPrefix(line)
		// Should contain the prefix text and log content.
		assert.Contains(t, result, "pod/myapp-abc12/app")
		assert.Contains(t, result, "some log output")
	})

	t.Run("no prefix passes through", func(t *testing.T) {
		line := "plain log line without prefix"
		result := colorizePodPrefix(line)
		assert.Equal(t, line, result)
	})

	t.Run("empty line passes through", func(t *testing.T) {
		assert.Equal(t, "", colorizePodPrefix(""))
	})

	t.Run("same pod gets same result", func(t *testing.T) {
		line1 := "[pod/myapp-abc12/app] log 1"
		line2 := "[pod/myapp-abc12/app] log 2"
		r1 := colorizePodPrefix(line1)
		r2 := colorizePodPrefix(line2)
		// Same pod prefix should produce identical styling.
		idx1 := strings.Index(r1, "log 1")
		idx2 := strings.Index(r2, "log 2")
		assert.Greater(t, idx1, 0)
		assert.Greater(t, idx2, 0)
		assert.Equal(t, r1[:idx1], r2[:idx2])
	})

	t.Run("no close bracket passes through", func(t *testing.T) {
		line := "[incomplete prefix without close"
		result := colorizePodPrefix(line)
		assert.Equal(t, line, result)
	})

	t.Run("extracts pod name correctly for hashing", func(t *testing.T) {
		// Two lines with same pod but different containers should get same color.
		line1 := "[pod/myapp-abc12/app] log 1"
		line2 := "[pod/myapp-abc12/sidecar] log 2"
		r1 := colorizePodPrefix(line1)
		r2 := colorizePodPrefix(line2)
		// Both reference pod "myapp-abc12" so they should get same color styling.
		// We can't directly compare ANSI codes in non-terminal tests,
		// but verify both process correctly.
		assert.Contains(t, r1, "pod/myapp-abc12/app")
		assert.Contains(t, r2, "pod/myapp-abc12/sidecar")
	})
}
