package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// w/b/e classified by rune, not by grapheme cluster, land on a combining mark
// or a joiner instead of the glyph it belongs to.
const (
	// "café bar", é decomposed as e + U+0301: c@0 a@1 f@2 é@3 ' '@4 b@5 a@6 r@7.
	decomposedAccentLine = "café bar"
	// digit + U+FE0F variation selector renders as one two-cell glyph: the
	// cluster spans cols 0-1, then a space at 2, then "team" at 3.
	variationSelectorLine = "1️ team"
	// ZWJ family emoji (three people joined) followed by a letter, so the
	// glyph and the "x" form one word: family@0 (2 cols) x@2 ' '@3 "end"@4.
	zwjFamilyLine = "\U0001F468" + zwjMark + "\U0001F469" + zwjMark + "\U0001F467x end"
	// U+200D zero-width joiner, spelled out in bytes: an invisible character
	// in a literal reads as a typo and staticcheck rejects it.
	zwjMark = "\xe2\x80\x8d"
)

func TestWordMotion_GraphemeClusters(t *testing.T) {
	tests := []struct {
		name string
		line string
		from int
		key  string
		want int
	}{
		{"w skips a decomposed accent as one character", decomposedAccentLine, 0, "w", 5},
		{"e lands on the decomposed accent's own column", decomposedAccentLine, 0, "e", 3},
		{"b returns to the start of the accented word", decomposedAccentLine, 5, "b", 0},

		{"w skips a variation-selector glyph as one character", variationSelectorLine, 0, "w", 3},
		{"e on a single-glyph word advances to the next word's end", variationSelectorLine, 0, "e", 6},
		{"b returns to the variation-selector glyph", variationSelectorLine, 3, "b", 0},

		{"w skips a ZWJ emoji family as one character", zwjFamilyLine, 0, "w", 4},
		{"e lands on the end of the ZWJ emoji's word", zwjFamilyLine, 0, "e", 2},
		{"b returns to the start of the ZWJ emoji's word", zwjFamilyLine, 4, "b", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wideRuneLogModel(tt.line, tt.from)
			ret, _, ok := m.handleLogMovementKey(keyMsg(tt.key))
			require.True(t, ok)
			assert.Equal(t, tt.want, ret.(Model).logView.visualCurCol)
		})
	}
}

// A zero-width cluster (tab) shares its column offset with the next visible
// character. Without the duplicate-offset guard in colIndex, w/W stays stuck.
func TestWordMotion_ZeroWidthTab(t *testing.T) {
	line := "\tfoo bar"
	assert.Equal(t, 8, nextWordStart(line, 0))
	assert.Equal(t, 8, nextWORDStart(line, 0))
}

// W/B/E share nextStart/wordEndWith/prevStart with w/b/e, so a WORD-motion
// regression would share the same broken column arithmetic.
func TestWORDMotion_GraphemeClusters(t *testing.T) {
	m := wideRuneLogModel(zwjFamilyLine, 0)
	ret, _, ok := m.handleLogMovementKey(keyMsg("W"))
	require.True(t, ok)
	assert.Equal(t, 4, ret.(Model).logView.visualCurCol)

	m = wideRuneLogModel(zwjFamilyLine, 0)
	ret, _, ok = m.handleLogMovementKey(keyMsg("E"))
	require.True(t, ok)
	assert.Equal(t, 2, ret.(Model).logView.visualCurCol)

	m = wideRuneLogModel(zwjFamilyLine, 4)
	ret, _, ok = m.handleLogMovementKey(keyMsg("B"))
	require.True(t, ok)
	assert.Equal(t, 0, ret.(Model).logView.visualCurCol)
}
