package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression coverage for #793 and #794: a wrapped sub-line only covers a
// slice of the source line's columns, so the selection and the block cursor
// must be addressed in that slice's space.

// wrapVisualLine is 50 columns wide and wraps to three 20/20/10 sub-lines at
// width 21 (1 column goes to the cursor gutter). Each run of letters marks the
// sub-line it belongs to, so a highlight on the wrong sub-line is visible in
// the assertion.
const wrapVisualLine = "aaaaaaaaaaaaaaaaaaaa" + "bbbbbbbbbbbbbbbbbbbb" + "cccccccccc"

// selectedRunes returns the runes of line that carry SelectedStyle, so a test
// can assert on the highlighted text rather than on escape-sequence bytes.
func selectedRunes(t *testing.T, line string) string {
	t.Helper()
	prefix, _, ok := strings.Cut(SelectedStyle.Render("\x00"), "\x00")
	require.True(t, ok, "SelectedStyle must not add visible characters")

	var out strings.Builder
	for rest := line; ; {
		i := strings.Index(rest, prefix)
		if i < 0 {
			return out.String()
		}
		rest = rest[i+len(prefix):]
		end := strings.Index(rest, "\x1b[m")
		if end < 0 {
			out.WriteString(ansi.Strip(rest))
			return out.String()
		}
		out.WriteString(ansi.Strip(rest[:end]))
		rest = rest[end:]
	}
}

func TestRenderWrappedLines_CharSelectionLandsOnOwningSubLine(t *testing.T) {
	// Anchor col 25, cursor col 34: ten "b" runes, entirely inside sub-line 2.
	rows, _, _ := renderWrappedLines([]string{wrapVisualLine}, 0, 10, 21, false, 0,
		0, 0, 0, 0, 'v', 25, 34, 0)
	require.Len(t, rows, 3)

	assert.Empty(t, selectedRunes(t, rows[0]), "sub-line 1 holds no selected column")
	assert.Equal(t, strings.Repeat("b", 10), selectedRunes(t, rows[1]))
	assert.Empty(t, selectedRunes(t, rows[2]), "sub-line 3 holds no selected column")
}

func TestRenderWrappedLines_CharSelectionSpansSubLines(t *testing.T) {
	// Anchor col 15, cursor col 24: five "a" runes then five "b" runes.
	rows, _, _ := renderWrappedLines([]string{wrapVisualLine}, 0, 10, 21, false, 0,
		0, 0, 0, 0, 'v', 15, 24, 0)
	require.Len(t, rows, 3)

	assert.Equal(t, strings.Repeat("a", 5), selectedRunes(t, rows[0]))
	assert.Equal(t, strings.Repeat("b", 5), selectedRunes(t, rows[1]))
	assert.Empty(t, selectedRunes(t, rows[2]))
}

func TestRenderWrappedLines_BlockSelectionLandsOnOwningSubLine(t *testing.T) {
	rows, _, _ := renderWrappedLines([]string{wrapVisualLine}, 0, 10, 21, false, 0,
		0, 0, 0, 0, 'B', 42, 44, 0)
	require.Len(t, rows, 3)

	assert.Empty(t, selectedRunes(t, rows[0]))
	assert.Empty(t, selectedRunes(t, rows[1]))
	assert.Equal(t, "ccc", selectedRunes(t, rows[2]))
}

func TestRenderWrappedLines_CursorRendersOnOwningSubLine(t *testing.T) {
	rows, cursorRow, cursorCol := renderWrappedLines([]string{wrapVisualLine}, 0, 10, 21, false, 0,
		0, -1, -1, -1, 0, 0, 45, 0)
	require.Len(t, rows, 3)

	assert.Equal(t, 2, cursorRow, "cursor column 45 sits on the third sub-line")
	assert.Equal(t, 1+5, cursorCol, "gutter plus the sub-line-local column")
	assert.Contains(t, rows[2], CursorBlockStyle.Render("c"))
	assert.NotContains(t, rows[0], CursorBlockStyle.Render(" "))
}

// The pod-prefix colorizer only belongs on the first sub-line. A continuation
// row that happens to open with a bracket is not a prefix, and running the
// colorizer over it sanitizes away the escapes the block cursor is made of.
func TestRenderWrappedLines_CursorSurvivesABracketedContinuation(t *testing.T) {
	line := strings.Repeat("a", 20) + "[INFO] tail"

	rows, cursorRow, _ := renderWrappedLines([]string{line}, 0, 10, 21, false, 0,
		0, -1, -1, -1, 0, 0, 22, 0)
	require.Len(t, rows, 2)
	require.Equal(t, 1, cursorRow)

	assert.Equal(t, " "+RenderCursorAtCol("[INFO] tail", 2), rows[1],
		"the continuation row carries the cursor and nothing else")
}

// The pod prefix belongs to the first sub-line whoever holds the cursor. Tying
// the colorizer to "not the cursor's line" dropped the color off the whole
// entry the moment the cursor moved onto a continuation row.
func TestRenderWrappedLines_PodPrefixSurvivesACursorOnALaterRow(t *testing.T) {
	line := "[pod/foo/bar] " + strings.Repeat("a", 40)
	wrapped := WrapLine(line, 20)
	require.Greater(t, len(wrapped), 1, "the fixture must wrap")

	rows, cursorRow, _ := renderWrappedLines([]string{line}, 0, 10, 21, false, 0,
		0, -1, -1, -1, 0, 0, 25, 0)
	require.Equal(t, 1, cursorRow, "column 25 sits on the second sub-line")

	assert.Equal(t, YamlCursorIndicatorStyle.Render("▎")+colorizePodPrefix(wrapped[0]), rows[0],
		"the prefix row keeps its color while the cursor sits below it")
}

func TestRenderWrappedLines_NoRowExceedsContentWidth(t *testing.T) {
	// A cursor parked on the last column of a sub-line that already fills the
	// row must not push the row past the width the border expects, or the
	// outer width guard truncates the cursor away. Reported as "the cursor
	// disappears until lfk restarts".
	for _, col := range []int{19, 20, 39, 40, 49, 55} {
		rows, _, _ := renderWrappedLines([]string{wrapVisualLine}, 0, 10, 21, false, 0,
			0, -1, -1, -1, 0, 0, col, 0)
		for i, row := range rows {
			assert.LessOrEqual(t, ansi.StringWidth(row), 21,
				"col %d row %d overflows the content width", col, i)
		}
	}

	// A line that wraps into exactly-full sub-lines leaves no cell to park a
	// cursor that ran past the end, which is the shape that truncated it away.
	flush := strings.Repeat("a", 40)
	for _, col := range []int{40, 45} {
		rows, cursorRow, _ := renderWrappedLines([]string{flush}, 0, 10, 21, false, 0,
			0, -1, -1, -1, 0, 0, col, 0)
		require.Equal(t, 1, cursorRow, "col %d lands on the final sub-line", col)
		for i, row := range rows {
			assert.LessOrEqual(t, ansi.StringWidth(row), 21,
				"col %d row %d overflows the content width", col, i)
		}
		assert.Contains(t, rows[1], CursorBlockStyle.Render("a"), "col %d keeps a visible cursor", col)
	}
}

func TestRenderWrappedLines_ParkedSelectionKeepsRowWidth(t *testing.T) {
	// The selection's parked cell overflows the same way the cursor's does
	// when the final sub-line already fills the row.
	flush := strings.Repeat("a", 40)
	rows, _, _ := renderWrappedLines([]string{flush}, 0, 10, 21, false, 0,
		0, 0, 0, 0, 'v', 40, 40, 0)
	require.Len(t, rows, 2)

	for i, row := range rows {
		assert.LessOrEqual(t, ansi.StringWidth(row), 21, "row %d overflows the content width", i)
	}
	assert.Equal(t, "a", selectedRunes(t, rows[1]), "the parked cell stays visible")
}

func TestRenderWrappedLines_SelectionOnEmptyLineShowsParkedCell(t *testing.T) {
	rows, _, _ := renderWrappedLines([]string{""}, 0, 10, 21, false, 0,
		0, 0, 0, 0, 'v', 0, 0, 0)
	require.Len(t, rows, 1)
	assert.Equal(t, " ", selectedRunes(t, rows[0]))
}
