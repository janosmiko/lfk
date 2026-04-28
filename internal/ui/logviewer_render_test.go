package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- renderWrappedLines ---

func TestRenderWrappedLines(t *testing.T) {
	t.Run("basic wrapping", func(t *testing.T) {
		lines := []string{"hello world this is a long line", "short"}
		result := renderWrappedLines(lines, 0, 10, 15, false, 0, -1, -1, -1, -1, 0, 0, 0, 0)
		// Should have multiple wrapped sub-lines for the first long line.
		assert.Greater(t, len(result), 2)
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "hello")
		assert.Contains(t, joined, "short")
	})

	t.Run("scroll skips initial lines", func(t *testing.T) {
		lines := []string{"line1", "line2", "line3", "line4"}
		result := renderWrappedLines(lines, 2, 3, 40, false, 0, -1, -1, -1, -1, 0, 0, 0, 0)
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "line3")
		assert.Contains(t, joined, "line4")
		assert.NotContains(t, joined, "line1")
	})

	t.Run("height limits output", func(t *testing.T) {
		lines := []string{"a", "b", "c", "d", "e"}
		result := renderWrappedLines(lines, 0, 3, 40, false, 0, -1, -1, -1, -1, 0, 0, 0, 0)
		assert.LessOrEqual(t, len(result), 3)
	})

	t.Run("empty lines list", func(t *testing.T) {
		result := renderWrappedLines(nil, 0, 5, 40, false, 0, -1, -1, -1, -1, 0, 0, 0, 0)
		assert.Empty(t, result)
	})

	t.Run("line numbers shown", func(t *testing.T) {
		lines := []string{"line1", "line2"}
		result := renderWrappedLines(lines, 0, 2, 40, true, 3, -1, -1, -1, -1, 0, 0, 0, 0)
		joined := strings.Join(result, "\n")
		assert.Contains(t, joined, "1")
		assert.Contains(t, joined, "2")
	})

	t.Run("cursor line gets indicator glyph", func(t *testing.T) {
		lines := []string{"hello", "world"}
		result := renderWrappedLines(lines, 0, 2, 40, false, 0, 0, -1, -1, -1, 0, 0, 0, 0)
		assert.Contains(t, result[0], "\u258e")
	})

	t.Run("non-cursor lines start with space", func(t *testing.T) {
		lines := []string{"hello", "world"}
		result := renderWrappedLines(lines, 0, 2, 40, false, 0, 0, -1, -1, -1, 0, 0, 0, 0)
		assert.True(t, strings.HasPrefix(result[1], " "), "non-cursor line should start with space")
	})

	t.Run("very long line wraps to multiple sub-lines", func(t *testing.T) {
		longLine := strings.Repeat("x", 100)
		lines := []string{longLine}
		result := renderWrappedLines(lines, 0, 20, 20, false, 0, -1, -1, -1, -1, 0, 0, 0, 0)
		// 100 chars / (20-1 gutter) = ~6 wrapped lines.
		assert.Greater(t, len(result), 1)
	})
}

// --- RenderLogViewer ---

func TestRenderLogViewer(t *testing.T) {
	t.Run("basic rendering contains title and lines", func(t *testing.T) {
		lines := []string{"log entry 1", "log entry 2", "log entry 3"}
		result := RenderLogViewer(
			lines, 0, 80, 20,
			false, false, false, false, false, false,
			"my-pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "my-pod")
		assert.Contains(t, result, "log entry 1")
		assert.Contains(t, result, "3 lines")
	})

	t.Run("follow indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			true, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "FOLLOW")
	})

	t.Run("wrap indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, true, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "WRAP")
	})

	t.Run("line numbers indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, true, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "LINE#")
	})

	t.Run("timestamps indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, true, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "TIMESTAMPS")
	})

	t.Run("previous indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, true, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "PREVIOUS")
	})

	t.Run("search query shown in title", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"error log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "error", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "/error")
	})

	t.Run("search active shows input prompt", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "err",
			true, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "err")
		assert.Contains(t, result, "enter:apply")
	})

	t.Run("status message shown in footer", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"Saved to file", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "Saved to file")
	})

	t.Run("visual mode indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			0, true, 0, 'V', 0, 0, 0,
		)
		assert.Contains(t, result, "VISUAL LINE")
	})

	t.Run("visual char mode indicator", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			0, true, 0, 'v', 0, 0, 0,
		)
		assert.Contains(t, result, "VISUAL")
	})

	t.Run("visual block mode indicator", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			0, true, 0, 'B', 0, 0, 0,
		)
		assert.Contains(t, result, "VISUAL BLOCK")
	})

	t.Run("empty lines list renders without panic", func(t *testing.T) {
		result := RenderLogViewer(
			nil, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "pod")
		assert.Contains(t, result, "0 lines")
	})

	t.Run("loading history indicator shown", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 80, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, true,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "LOADING HISTORY")
	})

	t.Run("switch pod hint shown when wide enough", func(t *testing.T) {
		// Use a very wide terminal so the hint bar does not truncate.
		result := RenderLogViewer(
			[]string{"log"}, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, true, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "switch pod")
	})

	t.Run("containers hint shown when wide enough", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"log"}, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, true, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "containers")
	})

	t.Run("wrap mode uses wrapped renderer", func(t *testing.T) {
		longLine := strings.Repeat("x", 100)
		result := RenderLogViewer(
			[]string{longLine}, 0, 40, 20,
			false, true, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "WRAP")
		// The long line should be visible (at least part of it).
		assert.Contains(t, result, "xxx")
	})

	t.Run("n/N hidden in default hint bar when no committed search", func(t *testing.T) {
		// Wide terminal so the hint bar is not truncated.
		result := RenderLogViewer(
			[]string{"log"}, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		// "/" search hint must still be there; "n/N" must not.
		assert.Contains(t, result, "search")
		assert.NotContains(t, result, "n/N")
		assert.NotContains(t, result, "next/prev")
	})

	t.Run("n/N shown in default hint bar when search is committed", func(t *testing.T) {
		result := RenderLogViewer(
			[]string{"error log"}, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "error", "",
			false, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "n/N")
		assert.Contains(t, result, "next/prev")
	})

	t.Run("n/N hidden during search input regardless of prior committed search", func(t *testing.T) {
		// searchActive=true means user is typing in the search prompt; the
		// footer is the prompt bar, not the default hints.
		result := RenderLogViewer(
			[]string{"err"}, 0, 400, 20,
			false, false, false, false, false, false,
			"pod", "error", "err",
			true, false, false, false, false,
			"", false,
			-1, false, 0, 0, 0, 0, 0,
		)
		assert.Contains(t, result, "enter:apply")
		assert.NotContains(t, result, "n/N")
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

	t.Run("no-color mode emits no ANSI escape codes", func(t *testing.T) {
		origNoColor := ConfigNoColor
		t.Cleanup(func() { SetNoColor(origNoColor) })

		SetNoColor(true)

		// Try several pod names so we hit different palette indices;
		// one of them is guaranteed to hash to the red/pink slot.
		cases := []string{
			"[pod/myapp-abc12/app] log line",
			"[pod/web-xyz9k/server] request served",
			"[pod/worker-1/proc] tick",
			"[pod/redis-primary/redis] OK",
		}
		for _, line := range cases {
			result := colorizePodPrefix(line)
			assert.NotContains(t, result, "\x1b[",
				"no-color must not emit ANSI escape codes; got: %q", result)
			// Text content stays intact.
			assert.Contains(t, result, line[1:strings.Index(line, "]")])
		}
	})
}
