package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// Cursor movement, selection, and the yank all count cell columns. While they
// counted runes instead, a CJK glyph or an emoji put them out of step: the
// highlight, the clipboard, and the cursor each landed somewhere else.

const (
	cjkLine   = "日本語text"      // 6 runes, 9 cells
	emojiLine = "a\U0001F600b" // 3 runes, 4 cells
	// U+200D zero-width joiner, spelled out in bytes: an invisible character in
	// a literal reads as a typo and staticcheck rejects it.
	zwj = "\xe2\x80\x8d"
	// A ZWJ family then an ASCII tail: 5 runes and 2 cells before the "x".
	zwjLine  = "\U0001F468" + zwj + "\U0001F469" + zwj + "\U0001F467x"
	combLine = "éclair" // 7 runes, 6 cells
)

func wideRuneLogModel(line string, col int) Model {
	return Model{
		mode: modeLogs,
		logView: logViewState{
			lines:        []string{line},
			cursor:       0,
			visualCurCol: col,
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}
}

// A motion must answer in the same columns the renderer draws in, so `l` over
// a double-width glyph advances two columns, not one.
func TestLogMotion_StepsWholeCharacters(t *testing.T) {
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
		{"h back over an emoji", emojiLine, 3, "h", 1},
		{"l past a combining mark", combLine, 0, "l", 1},
		{"h back onto a combining mark's letter", combLine, 1, "h", 0},
		// A mark with no character before it draws on the one after it, so
		// both share column 0 and l has to leave the line.
		{"l off a leading combining mark", "́x", 0, "l", 1},
		// Several runes, one glyph, two cells: the motion clears it in one step.
		{"l over a zwj emoji family", zwjLine, 0, "l", 2},
		{"h back over a zwj emoji family", zwjLine, 2, "h", 0},
	}
	require.Len(t, []rune(combLine), 7, "the fixture must be decomposed")
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wideRuneLogModel(tt.line, tt.from)
			ret, _, ok := m.handleLogMovementKey(keyMsg(tt.key))
			require.True(t, ok)
			assert.Equal(t, tt.want, ret.(Model).logView.visualCurCol)
		})
	}
}

// $ parks on the column the last character starts at. Width minus one would
// land on the right half of a trailing wide glyph.
func TestLogMotion_DollarLandsOnLastCharacter(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		{"trailing wide glyph", "text日本語", 8},
		// The last rune is the combining mark, which draws no cell of its own,
		// so a rune index parks the cursor past the visible text.
		{"trailing combining mark", "café", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wideRuneLogModel(tt.line, 0)
			ret, _, ok := m.handleLogMovementKey(keyMsg("$"))
			require.True(t, ok)
			assert.Equal(t, tt.want, ret.(Model).logView.visualCurCol)
		})
	}
}

// A count prefix is whatever the user typed, so the motion has to cost one
// walk over the line rather than one per counted step.
func TestLogMotion_HugeCountStopsAtTheLineEnd(t *testing.T) {
	line := strings.Repeat("日本", 500)
	done := make(chan int, 1)
	go func() { done <- stepCol(line, 0, 1_000_000_000) }()

	select {
	case got := <-done:
		assert.Equal(t, ui.LineWidth(line), got)
	case <-time.After(2 * time.Second):
		t.Fatal("stepCol did not finish: the count drives the loop, not the line")
	}
}

// Forward search resumes after the whole character under the cursor. Resuming
// one cell on lands inside a wide glyph, and the slice then starts a column
// earlier than the offset arithmetic assumes.
func TestLogSearch_NextMatchAfterAWideGlyph(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		query string
		from  int
		want  int
	}{
		{"repeated wide match", "界界", "界", 0, 2},
		{"ascii match after a wide glyph", "界ab", "b", 0, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wideRuneLogModel(tt.line, tt.from)
			m.findNextLogMatchForward(tt.query, 0)
			assert.Equal(t, tt.want, m.logView.visualCurCol)
		})
	}
}

// Backward search walks the line by cutting off the part it has already
// searched. A zero-width cluster at column 0 advanced the cut by nothing, so
// the same slice came back forever and the UI froze.
func TestLogSearch_BackwardScanTerminatesOnZeroWidthClusters(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		query string
		want  int
	}{
		{"leading combining mark", "́x", "́", 0},
		{"rightmost of two wide glyphs", "界界", "界", 2},
		{"ascii after a wide glyph", "界ab", "b", 3},
		{"no match", "abc", "z", -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			done := make(chan int, 1)
			go func() { done <- findLastMatchInStr(tt.line, tt.query) }()

			select {
			case got := <-done:
				assert.Equal(t, tt.want, got)
			case <-time.After(2 * time.Second):
				t.Fatal("findLastMatchInStr never returned: the cut stopped shrinking")
			}
		})
	}
}

// The clipboard has to hold the text the highlight covered. Slicing runes at a
// cell column copied a different substring on any line with a wide glyph.
func TestLogVisualYank_WideRunes(t *testing.T) {
	tests := []struct {
		name           string
		line           string
		anchor, cursor int
		want           string
	}{
		{"whole CJK run", cjkLine, 0, 5, "日本語"},
		{"across the CJK boundary", cjkLine, 4, 6, "語t"},
		{"ascii tail after CJK", cjkLine, 6, 8, "tex"},
		{"emoji alone", emojiLine, 1, 2, "\U0001F600"},
		{"emoji with neighbours", emojiLine, 0, 3, "a\U0001F600b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wideRuneLogModel(tt.line, tt.cursor)
			m.logView.visualMode = true
			m.logView.visualType = 'v'
			m.logView.visualStart = 0
			m.logView.visualCol = tt.anchor
			clip, _ := m.buildLogYankText()
			assert.Equal(t, tt.want, clip)
		})
	}
}

// The highlight and the cursor address the same columns as the motion, so the
// glyph the user lands on is the glyph that inverts.
func TestLogRender_CursorSitsOnTheWideGlyph(t *testing.T) {
	m := wideRuneLogModel(cjkLine, 2)
	m.width, m.height = 40, 12
	m.logView.follow = false

	view := m.viewLogs()
	assert.Contains(t, view, ui.CursorBlockStyle.Render("本"),
		"the cursor inverts the glyph at its column")
}

func TestLogRender_SelectionCoversTheWideGlyphs(t *testing.T) {
	m := wideRuneLogModel(cjkLine, 5)
	m.width, m.height = 40, 12
	m.logView.follow = false
	m.logView.visualMode = true
	m.logView.visualType = 'v'
	m.logView.visualStart = 0
	m.logView.visualCol = 0

	view := m.viewLogs()
	assert.Contains(t, view, ui.SelectedStyle.Render("日本語"),
		"the highlight covers exactly the selected glyphs")
}

// Tabs fill no cell as far as the renderer is concerned, so the log viewer
// expands them before a column is computed. Without that the cursor drifts one
// column left of the glyph for every tab earlier in the line.
func TestLogMotion_TabsAreExpandedBeforeColumns(t *testing.T) {
	m := wideRuneLogModel("a\tb", 0)
	line := m.logMotionLine(0)
	assert.NotContains(t, line, "\t", "tabs are expanded before motions see the line")
	assert.Equal(t, ui.LineWidth(line), len([]rune(line)), "expanded text is one cell per rune")

	ret, _, ok := m.handleLogMovementKey(keyMsg("$"))
	require.True(t, ok)
	assert.Equal(t, strings.LastIndex(line, "b"), ret.(Model).logView.visualCurCol)
}
