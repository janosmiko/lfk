package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// yamlTallLineModel puts the cursor on a value taller than the viewport, the
// shape the source-line scroll could never reach the end of. HEAD, MID, and
// TAIL each occur once, so a cursor assertion cannot pass from another column.
func yamlTallLineModel(t *testing.T) Model {
	t.Helper()
	m := baseModelNav()
	m.mode = modeYAML
	m.width, m.height = 80, 10
	m.yamlView.wrap = true
	m.yamlView.content = "spec:\n  value: HEAD" +
		strings.Repeat("a", 287) + "MID" + strings.Repeat("a", 310) + "TAIL"
	m.yamlView.sections = parseYAMLSections(m.yamlView.content)
	require.NotEmpty(t, m.yamlView.sections)

	visLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	require.Len(t, visLines, 2)
	m.yamlView.cursor = 1
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	return m
}

func TestViewYAML_WrappedLineScrollsToItsTail(t *testing.T) {
	m := yamlTallLineModel(t)

	head := stripANSI(m.viewYAML())
	require.Contains(t, head, "HEAD", "the line starts at the top of the viewport")
	require.NotContains(t, head, "TAIL", "the tail is taller than the viewport")

	ret, _ := m.handleYAMLKey(keyMsg("$"))
	tail := stripANSI(ret.(Model).viewYAML())

	assert.Contains(t, tail, "TAIL", "$ reaches the end of the wrapped line")
	assert.NotContains(t, tail, "HEAD", "the head scrolled off the top")
}

// The block cursor has to land on the sub-line that owns its column, and that
// sub-line has to be one the viewport renders.
func TestViewYAML_CursorSubLineStaysOnScreen(t *testing.T) {
	tests := []struct {
		name string
		col  int
		want string
	}{
		{"first sub-line", yamlFoldPrefixLen + 12, "D"},
		{"middle of the line", yamlFoldPrefixLen + 300, "M"},
		{"last character", yamlFoldPrefixLen + 616, "L"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := yamlTallLineModel(t)
			m.yamlView.visualCurCol = tt.col
			assert.Contains(t, m.viewYAML(), ui.CursorBlockStyle.Render(tt.want))
		})
	}
}

// Collapsing a fold shortens the visible lines without moving the cursor, so
// the renderer clamps it. The scroll offset has to be derived from the clamped
// cursor: the raw one is out of range, and out of range means no offset at all.
func TestViewYAML_CursorOffTheEndStillLandsOnScreen(t *testing.T) {
	m := yamlTallLineModel(t)
	m.yamlView.visualCurCol = yamlFoldPrefixLen + 616

	visLines, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	m.yamlView.cursor = len(visLines) + 3

	assert.Contains(t, m.viewYAML(), ui.CursorBlockStyle.Render("L"),
		"the cursor renders on its own sub-line after the clamp")
}
