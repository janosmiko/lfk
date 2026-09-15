package ui

import (
	"strconv"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// U+200D zero-width joiner, spelled out in bytes: an invisible character in a
// literal reads as a typo and staticcheck rejects it.
const zwj = "\xe2\x80\x8d"

// One glyph, two cells, five runes.
const zwjFamily = "\U0001F468" + zwj + "\U0001F469" + zwj + "\U0001F467"

func TestColumnOf(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		runeIdx int
		want    int
	}{
		{"ascii start", "hello", 0, 0},
		{"ascii middle", "hello", 3, 3},
		{"ascii past end", "hello", 9, 5},
		{"cjk second rune", "界a", 1, 2},
		{"cjk past both", "界a", 2, 3},
		{"emoji then ascii", "\U0001F600x", 1, 2},
		{"negative index", "hello", -1, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ColumnOf(tt.line, tt.runeIdx))
		})
	}
}

func TestRuneIndexAt(t *testing.T) {
	tests := []struct {
		name string
		line string
		col  int
		want int
	}{
		{"ascii start", "hello", 0, 0},
		{"ascii middle", "hello", 3, 3},
		{"ascii past end", "hello", 9, 5},
		{"cjk left half", "界a", 0, 0},
		// Column 1 sits inside the wide rune, which owns it.
		{"cjk right half", "界a", 1, 0},
		{"cjk after", "界a", 2, 1},
		{"cjk past end", "界a", 5, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, RuneIndexAt(tt.line, tt.col))
		})
	}
}

// ColumnOf and RuneIndexAt must round-trip on every rune boundary, or a motion
// that converts in and back out drifts off its own result.
func TestColumnRuneRoundTrip(t *testing.T) {
	for _, line := range []string{"hello", "界a界b", "a\U0001F600b", "日本語テキスト"} {
		for i := range []rune(line) {
			col := ColumnOf(line, i)
			assert.Equal(t, i, RuneIndexAt(line, col), "line %q rune %d", line, i)
		}
	}
}

func TestCutCols(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		start, end int
		want       string
	}{
		{"ascii slice", "hello", 1, 3, "el"},
		{"clamped end", "hello", 3, 99, "lo"},
		{"empty range", "hello", 3, 3, ""},
		{"inverted range", "hello", 4, 2, ""},
		{"negative start", "hello", -2, 2, "he"},
		{"whole wide rune", "界a", 0, 2, "界"},
		{"after wide rune", "界a", 2, 3, "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CutCols(tt.line, tt.start, tt.end))
		})
	}
}

func TestCharCols(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []int
	}{
		{"ascii", "abc", []int{0, 1, 2, 3}},
		{"wide runes", "界a界", []int{0, 2, 3, 5}},
		{"combining mark shares its letter's column", "éx", []int{0, 1, 2}},
		{"leading combining mark has no column of its own", "́x", []int{0, 1}},
		{"zwj emoji family is one glyph", zwjFamily, []int{0, 2}},
		{"regional indicator pair is one flag", "\U0001F1EF\U0001F1F5", []int{0, 2}},
		{"keycap sequence is one glyph", "1️⃣", []int{0, 2}},
		{"empty line", "", []int{0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CharCols(tt.line)
			assert.Equal(t, tt.want, got)
			assert.Equal(t, LineWidth(tt.line), got[len(got)-1],
				"the last entry is the width the renderer draws")
			assert.IsIncreasing(t, append([]int{-1}, got...),
				"a binary search needs the columns in order")
		})
	}
}

// CharCols feeds a binary search, and its columns are handed straight to the
// renderer, so each one has to be a boundary ansi.Cut agrees with.
func TestCharCols_MatchesTheRenderersBoundaries(t *testing.T) {
	corpus := []string{
		"", " ", "plain ascii text",
		"日本語text", "a\U0001F600b", "éclair", "́x",
		zwjFamily + "x",
		"\U0001F1EF\U0001F1F5\U0001F1E9\U0001F1EA",
		"tab\there", "ctrl\x07char", "mixed 界 á \U0001F600 end",
		"́̂̃", "界a\U0001F1EF\U0001F1F5é",
	}
	for _, line := range corpus {
		t.Run(strconv.Quote(line), func(t *testing.T) {
			cols := CharCols(line)
			require.NotEmpty(t, cols)
			assert.Equal(t, LineWidth(line), cols[len(cols)-1],
				"the columns have to add up to the width the renderer draws")
			assert.IsIncreasing(t, append([]int{-1}, cols...))
			for _, c := range cols {
				assert.Equal(t, c, SnapColStart(line, c), "column %d starts a character", c)
			}
		})
	}
}

// Keycaps are the one construct where ansi contradicts itself: StringWidth
// calls the glyph two cells, Cut and Truncate treat it as one. Columns follow
// StringWidth, which is what wrapping and the width guard use, so the cursor
// can sit a cell off on such a line. Delete this and put keycaps back in the
// corpus above once ansi agrees with itself.
func TestCharCols_KeycapWidthDisagreesUpstream(t *testing.T) {
	const keycap = "1️⃣"

	require.Equal(t, 2, LineWidth(keycap), "the measured width")
	require.Equal(t, 2, LineWidth(ansi.Cut(keycap, 0, 1)), "one cut column already carries the whole glyph")
	assert.NotEqual(t, 1, SnapColStart(keycap+"x", 2), "so column 2 is not a cut boundary")
}
