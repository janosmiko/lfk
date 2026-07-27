package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// addRowBlock returns the add-row portion of the rendered editor. Assertions
// must target this, not the whole overlay: the taint list above it also
// contains effect names. Anchored on the blank line the renderer joins the
// block with, not on the "add:" label — at narrow widths the label is itself
// truncated away.
func addRowBlock(t *testing.T, m Model) (block string, width int) {
	t.Helper()
	content, w, _ := m.renderOverlayTaintEditor()
	plain := stripANSI(content)
	sep := strings.LastIndex(plain, "\n\n")
	require.Positive(t, sep, "add-row must render while an input is focused")
	return plain[sep+2:], w
}

// The effect selector is a cycling control with no scroll of its own, so it
// must render in full no matter how long the key is. Before the fix the
// add-row was joined onto one line and truncated to a box sized from the taint
// LIST alone, so a realistic key pushed the effect off the edge as "eff~" —
// leaving the user unable to see which effect they were about to stage.
func TestRenderTaintEditorAddRow_LongKeyKeepsEffectVisible(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "a")
	m = taintKey(t, m, "node.kubernetes.io/very-long-taint", "tab", "true")

	block, _ := addRowBlock(t, m)

	assert.Contains(t, block, "effect <NoSchedule>", "effect must render in full")
	assert.NotContains(t, block, "eff~", "effect must never be the truncated field")
}

// A key too long for even a widened box truncates inside its own field, and
// the effect still renders — the row wraps rather than dropping fields.
func TestRenderTaintEditorAddRow_ExtremeKeyStillShowsEffect(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "a")
	m = taintKey(t, m, "example.com/"+strings.Repeat("x", 200))

	block, w := addRowBlock(t, m)

	assert.Contains(t, block, "effect <NoSchedule>", "effect must survive an unrenderable key")
	assert.LessOrEqual(t, w, m.width, "box must stay on screen")
	for line := range strings.SplitSeq(block, "\n") {
		assert.LessOrEqual(t, len([]rune(line)), w, "no line may overflow the box: %q", line)
	}
}

// The typing caret sits at the end of the focused field, so an over-long value
// truncates from the front — the user must see what they are typing.
func TestRenderTaintEditorAddRow_FocusedFieldKeepsCaretVisible(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m = taintKey(t, m, "a")
	m = taintKey(t, m, "k", "tab", "prefix-"+strings.Repeat("y", 120)+"-tail")

	block, _ := addRowBlock(t, m)

	assert.Contains(t, block, "-tail", "focused field must keep its tail (the caret end) visible")
	assert.Contains(t, block, "effect <NoSchedule>")
}

// The box grows to fit the add-row instead of sizing itself from the taint
// list alone, so a moderately long key needs no wrapping at all.
func TestRenderOverlayTaintEditor_BoxWidensForAddRow(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	_, listOnlyW, _ := m.renderOverlayTaintEditor()

	m = taintKey(t, m, "a")
	m = taintKey(t, m, "node-role.kubernetes.io/control-plane")
	block, addRowW := addRowBlock(t, m)

	require.Greater(t, addRowW, listOnlyW, "box must widen to fit the add-row")
	assert.LessOrEqual(t, addRowW, m.width, "box must stay on screen")
	assert.Contains(t, block, "node-role.kubernetes.io/control-plane", "key fits without truncation")
	assert.Equal(t, 1, strings.Count(strings.TrimSpace(block), "\n")+1,
		"a fitting add-row stays on one line")
}

// At a terminal narrow enough that a field's label alone exceeds the line
// budget, taintAddRowFields returns a field wider than innerW and the final
// safety-net truncation runs. That last cut must respect focus too: cutting the
// focused field from the right strips exactly the caret the per-field
// front-truncation just preserved.
func TestRenderTaintEditorAddRow_NarrowOverlayKeepsCaret(t *testing.T) {
	m := openTaintEditorLoaded(t, taintEditorTestModel())
	m.width = 20 // innerW lands well below the "add: key [" label width
	m = taintKey(t, m, "a")
	m = taintKey(t, m, "node.kubernetes.io/some-taint")

	block, _ := addRowBlock(t, m)

	assert.Contains(t, block, "█", "the caret must survive the safety-net truncation")
}
