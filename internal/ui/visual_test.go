package ui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- RenderCursorAtCol ---

func TestRenderCursorAtCol(t *testing.T) {
	tests := []struct {
		name       string
		styledLine string
		plainLine  string
		col        int
		wantSubstr []string
		wantAbsent []string
	}{
		{
			name:       "negative column returns styled line unchanged",
			styledLine: "hello world",
			plainLine:  "hello world",
			col:        -1,
			wantSubstr: []string{"hello world"},
		},
		{
			name:       "cursor at first character",
			styledLine: "hello",
			plainLine:  "hello",
			col:        0,
			wantSubstr: []string{"h", "ello"},
		},
		{
			name:       "cursor at middle character",
			styledLine: "hello",
			plainLine:  "hello",
			col:        2,
			wantSubstr: []string{"he", "l", "lo"},
		},
		{
			name:       "cursor at last character",
			styledLine: "hello",
			plainLine:  "hello",
			col:        4,
			wantSubstr: []string{"hell", "o"},
		},
		{
			name:       "cursor past end appends highlighted space",
			styledLine: "hello",
			plainLine:  "hello",
			col:        10,
			wantSubstr: []string{"hello", " "},
		},
		{
			name:       "cursor at exact end appends highlighted space",
			styledLine: "abc",
			plainLine:  "abc",
			col:        3,
			wantSubstr: []string{"abc", " "},
		},
		{
			name:       "empty line with cursor at 0 appends space",
			styledLine: "",
			plainLine:  "",
			col:        0,
			wantSubstr: []string{" "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderCursorAtCol(tt.styledLine, tt.plainLine, tt.col)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, result, absent, "result should not contain %q", absent)
			}
		})
	}
}

// Regression for kyverno-style log lines: producers that emit colored
// timestamps (`\x1b[90m2026-...`) reach the log viewer with the SGR sequence
// preserved (ConfigLogRenderAnsi defaults on). With rune-based slicing, any
// cursor column that landed inside an SGR sequence split it -- the cursor
// wrapper consumed the partial introducer and the remaining "NNm" leaked as
// literal text. The cursor must address visible columns and treat embedded
// SGR sequences as zero-width.
func TestRenderCursorAtCol_PreservesANSIInPlainLine(t *testing.T) {
	// Force ANSI so CursorBlockStyle emits real reverse-video codes.
	// In the default test profile lipgloss strips styles and the regression
	// becomes invisible.
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	line := "\x1b[90m2026-04-27T12:52:19Z TRC msg\x1b[0m"

	tests := []struct {
		name           string
		col            int
		wantCursorChar string
	}{
		{name: "cursor on first visible char", col: 0, wantCursorChar: "2"},
		{name: "cursor on second visible char", col: 1, wantCursorChar: "0"},
		{name: "cursor mid-timestamp", col: 4, wantCursorChar: "-"},
		{name: "cursor on T separator", col: 10, wantCursorChar: "T"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderCursorAtCol(line, line, tt.col)

			assert.Contains(t, result, "\x1b[90m",
				"original SGR opener must remain intact, not split by the cursor wrapper")

			assert.Contains(t, result, CursorBlockStyle.Render(tt.wantCursorChar),
				"cursor should highlight the visible character at column %d", tt.col)

			// A literal "[90m" preceded by anything other than ESC means the
			// SGR introducer was lost and the rest leaked as plain text.
			assert.NotRegexp(t, `(^|[^\x1b])\[90m`, result,
				"literal '[90m' would mean the SGR ESC was eaten")
			// Same regression check for the closing reset.
			assert.NotRegexp(t, `(^|[^\x1b])\[0m`, result,
				"literal '[0m' would mean the closing SGR ESC was eaten")
		})
	}
}

// Regression: line-mode visual selection (V) over a producer-colored log
// line had two distinct problems. First attempt re-asserted the selection
// style after each embedded reset, which kept the bg alive but caused
// producer foregrounds to collide with the selection bg (kyverno's
// `\x1b[34m` blue level on selection's blue bg => blue-on-blue invisible).
// Final fix strips producer ANSI inside the selection so the selection
// owns the visual presentation -- consistent legibility across themes and
// producers.
func TestRenderVisualSelection_LineModeKeepsAllTextLegible(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	// Mid-line "\x1b[34m" deliberately matches a common selection bg
	// color (blue 4) -- the regression scenario where producer fg and
	// selection bg collide and text disappears.
	line := "\x1b[90mtimestamp\x1b[0m \x1b[34mlevel\x1b[0m message"

	result := RenderVisualSelection(line, 'V', 0, 0, 0, 0, 0, 0, 0, 0)

	// All visible text must reach the output; ANSI-stripping the result
	// must yield the original visible characters with no producer SGR
	// payload leaking as text.
	stripped := ansi.Strip(result)
	require.NotEmpty(t, stripped, "selection render must produce visible content")

	for _, fragment := range []string{"timestamp", "level", "message"} {
		assert.Contains(t, stripped, fragment,
			"every visible field must appear in the rendered selection (got %q)", stripped)
	}
	assert.NotContains(t, stripped, "[34m",
		"producer SGR payload must not appear as plain text after stripping")

	// The selection style's own codes are present so the row reads as
	// selected -- bg/fg from SelectedStyle, not from the producer.
	openCodes := styleOpenCodes(SelectedStyle)
	require.NotEmpty(t, openCodes, "SelectedStyle must emit codes for this assertion to mean anything")
	assert.Contains(t, result, openCodes,
		"the selection's own open codes must drive the row's appearance")
}

// --- RenderVisualSelection ---

func TestRenderVisualSelection(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		visualType rune
		lineIdx    int
		selStart   int
		selEnd     int
		anchorLine int
		anchorCol  int
		cursorCol  int
		colStart   int
		colEnd     int
		wantSubstr []string
	}{
		{
			name:       "line visual mode highlights entire line",
			line:       "hello world",
			visualType: 'V',
			lineIdx:    5,
			selStart:   3,
			selEnd:     7,
			anchorLine: 3,
			anchorCol:  0,
			cursorCol:  0,
			colStart:   0,
			colEnd:     0,
			wantSubstr: []string{"hello world"},
		},
		{
			name:       "char visual mode single line",
			line:       "hello world",
			visualType: 'v',
			lineIdx:    5,
			selStart:   5,
			selEnd:     5,
			anchorLine: 5,
			anchorCol:  2,
			cursorCol:  7,
			colStart:   2,
			colEnd:     7,
			wantSubstr: []string{"he", "llo wo", "rld"},
		},
		{
			name:       "block visual mode highlights column range",
			line:       "hello world",
			visualType: 'B',
			lineIdx:    5,
			selStart:   3,
			selEnd:     7,
			anchorLine: 3,
			anchorCol:  2,
			cursorCol:  5,
			colStart:   2,
			colEnd:     5,
			wantSubstr: []string{"he", "llo ", "world"},
		},
		{
			name:       "char mode first line downward",
			line:       "start here",
			visualType: 'v',
			lineIdx:    3,
			selStart:   3,
			selEnd:     5,
			anchorLine: 3,
			anchorCol:  3,
			cursorCol:  5,
			colStart:   3,
			colEnd:     5,
			wantSubstr: []string{"sta", "rt here"},
		},
		{
			name:       "char mode last line downward",
			line:       "end here",
			visualType: 'v',
			lineIdx:    5,
			selStart:   3,
			selEnd:     5,
			anchorLine: 3,
			anchorCol:  3,
			cursorCol:  5,
			colStart:   3,
			colEnd:     5,
			wantSubstr: []string{"end he", "re"},
		},
		{
			name:       "char mode middle line fully highlighted",
			line:       "middle line",
			visualType: 'v',
			lineIdx:    4,
			selStart:   3,
			selEnd:     5,
			anchorLine: 3,
			anchorCol:  2,
			cursorCol:  6,
			colStart:   2,
			colEnd:     6,
			wantSubstr: []string{"middle line"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RenderVisualSelection(tt.line, tt.visualType, tt.lineIdx,
				tt.selStart, tt.selEnd, tt.anchorLine, tt.anchorCol, tt.cursorCol,
				tt.colStart, tt.colEnd)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}
}

// --- highlightColumnRange ---

func TestHighlightColumnRange(t *testing.T) {
	tests := []struct {
		name       string
		line       string
		colStart   int
		colEnd     int
		wantSubstr []string
	}{
		{
			name:       "full line highlight",
			line:       "hello",
			colStart:   0,
			colEnd:     5,
			wantSubstr: []string{"hello"},
		},
		{
			name:       "partial highlight in middle",
			line:       "hello world",
			colStart:   3,
			colEnd:     8,
			wantSubstr: []string{"hel", "lo wo", "rld"},
		},
		{
			name:       "highlight at start",
			line:       "hello",
			colStart:   0,
			colEnd:     3,
			wantSubstr: []string{"hel", "lo"},
		},
		{
			name:       "highlight at end",
			line:       "hello",
			colStart:   3,
			colEnd:     5,
			wantSubstr: []string{"hel", "lo"},
		},
		{
			name:       "selection beyond line length shows padding",
			line:       "hi",
			colStart:   5,
			colEnd:     10,
			wantSubstr: []string{"hi", " "},
		},
		{
			name:       "negative colStart clamped to 0",
			line:       "hello",
			colStart:   -3,
			colEnd:     3,
			wantSubstr: []string{"hel", "lo"},
		},
		{
			name:       "colEnd beyond line clamped",
			line:       "hi",
			colStart:   0,
			colEnd:     10,
			wantSubstr: []string{"hi"},
		},
		{
			name:       "colEnd less than colStart returns unmodified",
			line:       "hello",
			colStart:   5,
			colEnd:     3,
			wantSubstr: []string{"hello"},
		},
		{
			name:       "empty line with selection beyond",
			line:       "",
			colStart:   0,
			colEnd:     5,
			wantSubstr: []string{" "},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := highlightColumnRange(tt.line, ansi.StringWidth(tt.line), tt.colStart, tt.colEnd)
			for _, sub := range tt.wantSubstr {
				assert.Contains(t, result, sub, "result should contain %q", sub)
			}
		})
	}
}

// --- renderBlockSelection ---

func TestRenderBlockSelection(t *testing.T) {
	t.Run("block selection highlights columns inclusively", func(t *testing.T) {
		line := "abcdefghij"
		result := renderBlockSelection(line, ansi.StringWidth(line), 2, 5)
		// Columns 2 through 5 inclusive should be highlighted.
		assert.Contains(t, result, "ab")
		assert.Contains(t, result, "cdef")
		assert.Contains(t, result, "ghij")
	})
}

// --- renderCharSelection ---

func TestRenderCharSelection(t *testing.T) {
	t.Run("single line selection", func(t *testing.T) {
		line := "hello world"
		result := renderCharSelection(line, ansi.StringWidth(line), 5, 5, 5, 5, 2, 7)
		// Single line: highlight between cols 2 and 7.
		assert.Contains(t, result, "he")
		assert.Contains(t, result, "llo wo")
		assert.Contains(t, result, "rld")
	})

	t.Run("multi-line start line downward", func(t *testing.T) {
		line := "start line content"
		result := renderCharSelection(line, ansi.StringWidth(line), 3, 3, 7, 3, 5, 8)
		// Line at selStart: highlight from anchorCol (5) to end.
		assert.True(t, strings.Contains(result, "start"), "should contain pre-selection text")
	})

	t.Run("multi-line middle line", func(t *testing.T) {
		line := "fully selected"
		result := renderCharSelection(line, ansi.StringWidth(line), 5, 3, 7, 3, 2, 8)
		// Middle line: fully selected.
		assert.Contains(t, result, "fully selected")
	})
}

// Regression: char/block visual selection now treats anchorCol/cursorCol as
// visual columns and preserves embedded ANSI on the unselected segments while
// stripping it inside the selection (so producer fg can't collide with
// selection bg). On a kyverno-style line, selecting visual cols 0–8 should
// highlight the timestamp and leave the rest of the row in its producer
// colors with no broken SGR fragments leaking as plain text.
func TestHighlightColumnRange_ANSIAware(t *testing.T) {
	originalProfile := lipgloss.DefaultRenderer().ColorProfile()
	t.Cleanup(func() { lipgloss.DefaultRenderer().SetColorProfile(originalProfile) })
	lipgloss.DefaultRenderer().SetColorProfile(termenv.ANSI)

	line := "\x1b[90mtimestamp\x1b[0m \x1b[34mlevel\x1b[0m message"
	width := ansi.StringWidth(line)

	result := highlightColumnRange(line, width, 0, 9) // selects "timestamp"

	stripped := ansi.Strip(result)
	assert.Equal(t, "timestamp level message", stripped,
		"stripped result must equal stripped original — no characters lost or duplicated")

	assert.NotRegexp(t, `(^|[^\x1b])\[34m`, result,
		"producer SGR introducer must remain intact across the selection split")
	assert.NotRegexp(t, `(^|[^\x1b])\[0m`, result,
		"closing reset must keep its leading ESC byte")
}
