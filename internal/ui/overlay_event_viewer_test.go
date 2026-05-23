package ui

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// A short line that fits within contentW must come back unchanged —
// no padding, no extra newlines, no split.
func TestWrapEventLine_ShortLineUnchanged(t *testing.T) {
	t.Parallel()
	got := WrapEventLine("hello", 80, 10)
	assert.Equal(t, []string{"hello"}, got)
}

// The first physical line consumes the full contentW; each subsequent
// line is prefixed with hangingIndent spaces so wrapped message text
// stays under the message column.
func TestWrapEventLine_HangingIndent(t *testing.T) {
	t.Parallel()
	// Line: 38-col prefix (e.g. "2m       Warning  FailedScheduling    ")
	// followed by 60 chars of message → total 98. With contentW=50 and
	// hangingIndent=38, the first sub-line takes 50 chars, leaving 48
	// of message; each continuation fits (50 - 38) = 12 chars of
	// message preceded by 38 spaces.
	prefix := strings.Repeat("x", 38)
	msg := strings.Repeat("a", 60)
	line := prefix + msg

	got := WrapEventLine(line, 50, 38)
	pad := strings.Repeat(" ", 38)

	assert.Equal(t, prefix+strings.Repeat("a", 12), got[0],
		"first line keeps the full prefix and uses contentW chars")
	for i := 1; i < len(got); i++ {
		assert.True(t, strings.HasPrefix(got[i], pad),
			"continuation %d must start with hangingIndent spaces, got %q", i, got[i])
		assert.LessOrEqual(t, len([]rune(got[i])), 50,
			"continuation %d must not exceed contentW", i)
	}
	// All message chars must be present in concatenation order.
	var sb strings.Builder
	sb.WriteString(got[0])
	for _, l := range got[1:] {
		sb.WriteString(strings.TrimPrefix(l, pad))
	}
	assert.Equal(t, line, sb.String())
}

// If the indent is too wide to leave any meaningful continuation room
// (less than 8 chars), fall back to flush-left wrap to stay readable
// on narrow terminals.
func TestWrapEventLine_NarrowTerminalFallsBackFlushLeft(t *testing.T) {
	t.Parallel()
	long := strings.Repeat("a", 100)
	got := WrapEventLine(long, 20, 38) // 38 indent in 20-col content → fall back

	for i, l := range got {
		assert.False(t, strings.HasPrefix(l, "  "),
			"on narrow terminals continuation %d must NOT be padded, got %q", i, l)
	}
}

func TestWrapEventLine_DegenerateInputs(t *testing.T) {
	t.Parallel()
	assert.Equal(t, []string{""}, WrapEventLine("", 80, 10))
	assert.Equal(t, []string{"abc"}, WrapEventLine("abc", 0, 10))
	// Negative indent is treated as 0.
	got := WrapEventLine(strings.Repeat("a", 50), 20, -5)
	assert.NotEmpty(t, got)
}

// V-line visual selection used to collapse the wrap and render the
// event as a single truncated line — confusing because exiting visual
// mode brought wrap back. With this fix the selection follows the
// wrapped block: every physical sub-line is fully highlighted.
func TestRenderEventViewer_VisualLineModeKeepsWrap(t *testing.T) {
	t.Parallel()
	prefix := strings.Repeat("x", 38)
	msg := strings.Repeat("a", 200)
	p := EventViewerParams{
		Lines:         []string{prefix + msg},
		Width:         60,
		Height:        20,
		Wrap:          true,
		HangingIndent: 38,
		Cursor:        0,
		VisualMode:    'V',
		VisualStart:   0,
	}
	out := RenderEventViewer(p)
	// Continuation chunk (38-space indent + message piece) must still
	// appear in the rendered output — proving the wrap survived the
	// selection rendering rather than collapsing to a single line.
	pad := strings.Repeat(" ", 38)
	assert.Contains(t, out, pad+strings.Repeat("a", 12))
}

// Char-mode (v) selection on a wrapped event keeps the column-based
// semantics — it deliberately collapses to a single truncated line so
// the highlight columns map deterministically. Guard against future
// changes that accidentally enable wrap for char mode.
func TestRenderEventViewer_CharModeCollapsesToSingleLine(t *testing.T) {
	t.Parallel()
	prefix := strings.Repeat("x", 38)
	msg := strings.Repeat("a", 200)
	p := EventViewerParams{
		Lines:         []string{prefix + msg},
		Width:         60,
		Height:        20,
		Wrap:          true,
		HangingIndent: 38,
		Cursor:        0,
		VisualMode:    'v',
		VisualStart:   0,
		VisualCol:     0,
		CursorCol:     5,
	}
	out := RenderEventViewer(p)
	pad := strings.Repeat(" ", 38)
	assert.NotContains(t, out, pad+strings.Repeat("a", 12),
		"char-mode selection should not produce indented continuation")
}

// Regression guard for the two issues raised on #263 in the events
// overlay: each physical sub-line is prefixed with the gutter so the
// continuation lines do not appear to shift left when the cursor
// lands on the event.
func TestRenderEventViewer_WrappedContinuationsHaveGutter(t *testing.T) {
	t.Parallel()
	prefix := strings.Repeat("x", 38)
	msg := strings.Repeat("a", 200)
	p := EventViewerParams{
		Lines:         []string{prefix + msg},
		Width:         60, // contentW = 53
		Height:        20,
		Wrap:          true,
		HangingIndent: 38,
		Cursor:        0,
	}
	out := RenderEventViewer(p)

	// Every line of the rendered block that contains message text must
	// start with the cursor gutter "▎" (or the surrounding chrome).
	// We can't easily assert the styled escape sequence, but every
	// physical sub-line should be present in the output.
	assert.Contains(t, out, prefix)
	// Continuation chunks (`pad + 12-or-fewer a's`) should appear.
	pad := strings.Repeat(" ", 38)
	assert.Contains(t, out, pad+strings.Repeat("a", 12))
}
