package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// The YAML viewer shares the motion, selection, and yank code with the log
// viewer, so it has to agree on cell columns too. Its columns carry the fold
// prefix, which the yank drops again.

func wideRuneYAMLModel(t *testing.T, line string, col int) Model {
	t.Helper()
	m := baseModelNav()
	m.mode = modeYAML
	m.width, m.height = 80, 24
	// Parsed sections are what make buildVisibleLines add the fold prefix the
	// column math assumes, so the document needs a real key to hang them on.
	m.yamlView.content = "spec:\n  value: " + line
	m.yamlView.sections = parseYAMLSections(m.yamlView.content)
	require.NotEmpty(t, m.yamlView.sections, "the fixture must produce fold sections")

	visible, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	require.Len(t, visible, 2)
	m.yamlView.cursor = 1
	m.yamlView.visualCurCol = ui.LineWidth(visible[1]) - ui.LineWidth(line) + col
	return m
}

// yamlValuePrefixWidth is the number of columns before the test line's value
// on the second visible row.
func yamlValuePrefixWidth(t *testing.T) int {
	t.Helper()
	m := wideRuneYAMLModel(t, "", 0)
	visible, _ := buildVisibleLines(m.yamlView.content, m.yamlView.sections, m.yamlView.collapsed)
	return ui.LineWidth(visible[1])
}

func TestYAMLMotion_StepsWholeCharacters(t *testing.T) {
	tests := []struct {
		name string
		line string
		from int
		key  string
		want int
	}{
		{"l over a CJK glyph", cjkLine, 0, "l", 2},
		{"l onto ASCII after CJK", cjkLine, 6, "l", 7},
		{"h back over a CJK glyph", cjkLine, 6, "h", 4},
		{"l over an emoji", emojiLine, 1, "l", 3},
		{"h steps back into the key", cjkLine, 0, "h", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := yamlValuePrefixWidth(t)
			m := wideRuneYAMLModel(t, tt.line, tt.from)
			ret, _ := m.handleYAMLKey(keyMsg(tt.key))
			assert.Equal(t, base+tt.want, ret.(Model).yamlView.visualCurCol)
		})
	}
}

func TestYAMLVisualYank_WideRunes(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		anchor, cursor int
		want           string
	}{
		{"whole CJK run", cjkLine, 0, 5, "日本語"},
		{"across the CJK boundary", cjkLine, 4, 6, "語t"},
		{"emoji alone", emojiLine, 1, 2, "\U0001F600"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			base := yamlValuePrefixWidth(t)
			m := wideRuneYAMLModel(t, tt.line, tt.cursor)
			m.yamlView.visualMode = true
			m.yamlView.visualType = 'v'
			m.yamlView.visualStart = m.yamlView.cursor
			m.yamlView.visualCol = base + tt.anchor

			clip, _ := m.buildYAMLYankText()
			assert.Equal(t, tt.want, clip)

			ret, _ := m.handleYAMLVisualCopy()
			require.False(t, ret.(Model).yamlView.visualMode, "yank leaves visual mode")
		})
	}
}

// The block cursor has to invert the glyph its column names, not half of it.
func TestYAMLRender_CursorSitsOnTheWideGlyph(t *testing.T) {
	m := wideRuneYAMLModel(t, cjkLine, 2)
	view := m.viewYAML()
	assert.Contains(t, view, ui.CursorBlockStyle.Render("本"))
}
