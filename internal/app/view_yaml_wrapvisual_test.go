package app

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// wrapCtx wraps to 19-column sub-lines: contentWidth 28 less the 7-column
// gutter and the 2-column continuation indent. Document columns carry the
// fold prefix, which the renderer subtracts.
func wrapCtx() yamlRenderCtx {
	return yamlRenderCtx{
		mapping:      []int{0},
		gutterWidth:  3,
		contentWidth: 28,
		maxLines:     10,
		yamlCursor:   0,
		wrap:         true,
		currentMatch: -1,
		matchSet:     map[int]bool{},
		selStart:     -1,
		selEnd:       -1,
	}
}

const yamlWrapLine = "aaaaaaaaaaaaaaaaaaaa" + "bbbbbbbbbbbbbbbbbbbb" + "cccccccccc"

// yamlCursorCell returns the row and visible column the block cursor sits at.
// Asserting only that some row carries the cursor passes even when it lands on
// the wrong column of a row made of one repeated character.
func yamlCursorCell(t *testing.T, rows []string) (int, int) {
	t.Helper()
	open, _, ok := strings.Cut(ui.CursorBlockStyle.Render("\x00"), "\x00")
	require.True(t, ok, "the cursor style must not add visible characters")

	for i, row := range rows {
		if before, _, found := strings.Cut(row, open); found {
			return i, lipgloss.Width(before)
		}
	}
	return -1, -1
}

func TestRenderYAMLWrappedLine_SelectionLandsOnOwningSubLine(t *testing.T) {
	ctx := wrapCtx()
	ctx.visualMode = true
	ctx.visualType = 'v'
	ctx.selStart, ctx.selEnd = 0, 0
	ctx.visualStart = 0
	// Content columns 25..34: ten "b" runes on the second sub-line.
	ctx.visualCol = 25 + yamlFoldPrefixLen
	ctx.cursorCol = 34 + yamlFoldPrefixLen
	ctx.visualCurCol = ctx.cursorCol

	rows := renderYAMLWrappedLine(nil, yamlWrapLine, "  ", 0, 0, ctx)
	require.Len(t, rows, 3)

	assert.Contains(t, rows[1], ui.SelectedStyle.Render(strings.Repeat("b", 10)))
	assert.NotContains(t, rows[0], ui.SelectedStyle.Render(" "), "no stray selected cell")
	assert.NotContains(t, rows[2], ui.SelectedStyle.Render(" "), "no stray selected cell")
}

func TestRenderYAMLWrappedLine_CursorRendersOnOwningSubLine(t *testing.T) {
	ctx := wrapCtx()
	// Content column 45: the sixth "c", on the third sub-line.
	ctx.cursorCol = 45 + yamlFoldPrefixLen
	ctx.visualCurCol = ctx.cursorCol

	rows := renderYAMLWrappedLine(nil, yamlWrapLine, "  ", 0, 0, ctx)
	require.Len(t, rows, 3)

	row, col := yamlCursorCell(t, rows)
	assert.Equal(t, 2, row, "column 45 sits on the third sub-line")
	assert.Equal(t, 9+7, col, "the 7-column gutter plus the 2-column continuation indent, then column 45 less the sub-line's base of 38")
	assert.Contains(t, rows[2], ui.CursorBlockStyle.Render("c"))
}

func TestRenderYAMLWrappedLine_ParkedCursorKeepsRowWidth(t *testing.T) {
	// Continuation sub-lines carry an extra indent, and the YAML l motion is
	// unclamped, so both the sub-line itself and a cursor parked past its end
	// can outgrow the row and lose their tail to the outer width guard.
	ctx := wrapCtx()
	ctx.cursorCol = 90 + yamlFoldPrefixLen
	ctx.visualCurCol = ctx.cursorCol

	// The cursor cell is an "a" when it sits on the last column and a space
	// when it parks after it, so match the style rather than the character.
	cursorMark, _, ok := strings.Cut(ui.CursorBlockStyle.Render("\x00"), "\x00")
	require.True(t, ok)

	for _, n := range []int{38, 40, 42, 60} {
		rows := renderYAMLWrappedLine(nil, strings.Repeat("a", n), "  ", 0, 0, ctx)
		require.NotEmpty(t, rows)
		for i, row := range rows {
			assert.LessOrEqual(t, lipgloss.Width(row), ctx.contentWidth,
				"line of %d: row %d overflows the content width", n, i)
		}
		assert.Contains(t, rows[len(rows)-1], cursorMark,
			"line of %d: the cursor stays visible", n)
	}
}

// The cursor goes on the row that owns its column, however far down the line
// that is. Keeping it visible is the viewport's job, not this renderer's.
func TestRenderYAMLWrappedLine_CursorGoesOnItsOwnRow(t *testing.T) {
	ctx := wrapCtx()
	ctx.maxLines = 30
	ctx.cursorCol = 300 + yamlFoldPrefixLen
	ctx.visualCurCol = ctx.cursorCol

	rows := renderYAMLWrappedLine(nil, strings.Repeat("a", 400), "  ", 0, 0, ctx)
	require.Len(t, rows, 22, "400 columns wrap to 22 rows of 19")

	row, col := yamlCursorCell(t, rows)
	assert.Equal(t, 15, row, "column 300 falls on the 16th row")
	assert.Equal(t, 9+15, col, "the 9-column continuation prefix plus column 300 less the row's base of 285")
}

func TestRenderYAMLWrappedLine_CursorOnFirstSubLineStillWorks(t *testing.T) {
	ctx := wrapCtx()
	ctx.cursorCol = 3 + yamlFoldPrefixLen
	ctx.visualCurCol = ctx.cursorCol

	rows := renderYAMLWrappedLine(nil, yamlWrapLine, "  ", 0, 0, ctx)
	require.Len(t, rows, 3)

	row, col := yamlCursorCell(t, rows)
	assert.Equal(t, 0, row)
	assert.Equal(t, 7+3, col, "the 7-column gutter plus column 3")
	assert.Contains(t, rows[0], ui.CursorBlockStyle.Render("a"))
}
