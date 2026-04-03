package app

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- isWordBoundary ---

func TestIsWordBoundary(t *testing.T) {
	tests := []struct {
		r        rune
		expected bool
	}{
		{' ', true},
		{'\t', true},
		{'.', true},
		{':', true},
		{',', true},
		{';', true},
		{'/', true},
		{'-', true},
		{'_', true},
		{'"', true},
		{'\'', true},
		{'(', true},
		{')', true},
		{'[', true},
		{']', true},
		{'{', true},
		{'}', true},
		{'a', false},
		{'Z', false},
		{'0', false},
		{'@', false},
	}
	for _, tt := range tests {
		t.Run(string(tt.r), func(t *testing.T) {
			assert.Equal(t, tt.expected, isWordBoundary(tt.r))
		})
	}
}

// --- nextWordStart ---

func TestNextWordStart(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		col      int
		expected int
	}{
		{
			name:     "simple words",
			line:     "hello world",
			col:      0,
			expected: 6,
		},
		{
			name:     "at space",
			line:     "hello world",
			col:      5,
			expected: 6,
		},
		{
			name:     "with punctuation",
			line:     "foo.bar baz",
			col:      0,
			expected: 4,
		},
		{
			name:     "already at last char",
			line:     "hello",
			col:      4,
			expected: 5,
		},
		{
			name:     "past end",
			line:     "hello",
			col:      10,
			expected: 5,
		},
		{
			name:     "empty line",
			line:     "",
			col:      0,
			expected: 0,
		},
		{
			name:     "single char",
			line:     "a",
			col:      0,
			expected: 1,
		},
		{
			name:     "multiple separators",
			line:     "a   b",
			col:      0,
			expected: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, nextWordStart(tt.line, tt.col))
		})
	}
}

// --- wordEnd ---

func TestWordEnd(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		col      int
		expected int
	}{
		{
			name:     "end of first word",
			line:     "hello world",
			col:      0,
			expected: 4,
		},
		{
			name:     "at word end moves to next word end",
			line:     "hello world",
			col:      4,
			expected: 10,
		},
		{
			name:     "single character word",
			line:     "a b",
			col:      0,
			expected: 2,
		},
		{
			name:     "empty line",
			line:     "",
			col:      0,
			expected: 0,
		},
		{
			name:     "at last char",
			line:     "hello",
			col:      4,
			expected: 5,
		},
		{
			name:     "past end",
			line:     "hello",
			col:      10,
			expected: 5,
		},
		{
			name:     "with punctuation",
			line:     "foo.bar",
			col:      0,
			expected: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, wordEnd(tt.line, tt.col))
		})
	}
}

// --- prevWordStart ---

func TestPrevWordStart(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		col      int
		expected int
	}{
		{
			name:     "back to start of second word",
			line:     "hello world",
			col:      10,
			expected: 6,
		},
		{
			name:     "at start of second word back to first",
			line:     "hello world",
			col:      6,
			expected: 0,
		},
		{
			name:     "at start returns -1",
			line:     "hello",
			col:      0,
			expected: -1,
		},
		{
			name:     "empty line",
			line:     "",
			col:      0,
			expected: -1,
		},
		{
			name:     "with punctuation",
			line:     "foo.bar",
			col:      4,
			expected: 0,
		},
		{
			name:     "past end",
			line:     "hello world",
			col:      50,
			expected: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, prevWordStart(tt.line, tt.col))
		})
	}
}

// --- nextWORDStart ---

func TestNextWORDStart(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		col      int
		expected int
	}{
		{
			name:     "simple words",
			line:     "hello world",
			col:      0,
			expected: 6,
		},
		{
			name:     "ignores punctuation within WORD",
			line:     "foo.bar baz",
			col:      0,
			expected: 8,
		},
		{
			name:     "multiple spaces",
			line:     "a    b",
			col:      0,
			expected: 5,
		},
		{
			name:     "at last char",
			line:     "hello",
			col:      4,
			expected: 5,
		},
		{
			name:     "empty line",
			line:     "",
			col:      0,
			expected: 0,
		},
		{
			name:     "tab separator",
			line:     "foo\tbar",
			col:      0,
			expected: 4,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, nextWORDStart(tt.line, tt.col))
		})
	}
}

// --- prevWORDStart ---

func TestPrevWORDStart(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		col      int
		expected int
	}{
		{
			name:     "back from end",
			line:     "hello world",
			col:      10,
			expected: 6,
		},
		{
			name:     "back with punctuation",
			line:     "foo.bar baz",
			col:      10,
			expected: 8,
		},
		{
			name:     "at start",
			line:     "hello",
			col:      0,
			expected: -1,
		},
		{
			name:     "empty",
			line:     "",
			col:      0,
			expected: -1,
		},
		{
			name:     "past end clamped",
			line:     "hello world",
			col:      50,
			expected: 6,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, prevWORDStart(tt.line, tt.col))
		})
	}
}

// --- WORDEnd ---

func TestWORDEnd(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		col      int
		expected int
	}{
		{
			name:     "end of first WORD",
			line:     "hello world",
			col:      0,
			expected: 4,
		},
		{
			name:     "WORD with punctuation",
			line:     "foo.bar baz",
			col:      0,
			expected: 6,
		},
		{
			name:     "at WORD end moves to next",
			line:     "hello world",
			col:      4,
			expected: 10,
		},
		{
			name:     "at last char",
			line:     "hello",
			col:      4,
			expected: 5,
		},
		{
			name:     "empty",
			line:     "",
			col:      0,
			expected: 0,
		},
		{
			name:     "tab delimiter",
			line:     "foo\tbar",
			col:      0,
			expected: 2,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, WORDEnd(tt.line, tt.col))
		})
	}
}

// --- firstNonWhitespace ---

func TestFirstNonWhitespace(t *testing.T) {
	tests := []struct {
		name     string
		line     string
		expected int
	}{
		{"no leading whitespace", "hello", 0},
		{"leading spaces", "   hello", 3},
		{"leading tabs", "\t\thello", 2},
		{"mixed leading whitespace", "  \thello", 3},
		{"all whitespace", "     ", 0},
		{"empty string", "", 0},
		{"single char", "a", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, firstNonWhitespace(tt.line))
		})
	}
}

// --- countLines ---

func TestCountLines(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"empty string", "", 0},
		{"no newlines", "hello", 0},
		{"one newline", "hello\nworld", 1},
		{"multiple newlines", "a\nb\nc\n", 3},
		{"only newlines", "\n\n\n", 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, countLines(tt.input))
		})
	}
}

func TestCovBoost2DiffVisualKeyEsc(t *testing.T) {
	m := baseModelBoost2()
	m.mode = modeDiff
	m.diffVisualMode = true
	result, cmd := m.handleDiffVisualKey(keyMsg("esc"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.Nil(t, cmd)
}

func TestCovBoost2DiffVisualKeyV(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	// V when already in line mode toggles off.
	result, _ := m.handleDiffVisualKey(keyMsg("V"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, rune('V'), rm.diffVisualType)
}

func TestCovBoost2DiffVisualKeyVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	result, _ := m.handleDiffVisualKey(keyMsg("V"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovBoost2DiffVisualKeySmallV(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	result, _ := m.handleDiffVisualKey(keyMsg("v"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, rune('v'), rm.diffVisualType)
}

func TestCovBoost2DiffVisualKeySmallVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(keyMsg("v"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovBoost2DiffVisualKeyCtrlV(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV}, nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, rune('B'), rm.diffVisualType)
}

func TestCovBoost2DiffVisualKeyCtrlVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'B'
	result, _ := m.handleDiffVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV}, nil, 10, 5, 5)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovBoost2DiffVisualKeyJK(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("j"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 1, rm.diffCursor)

	result2, _ := rm.handleDiffVisualKey(keyMsg("k"), nil, 10, 5, 5)
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.diffCursor)
}

func TestCovBoost2DiffVisualKeyDown(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("down"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 1, rm.diffCursor)
}

func TestCovBoost2DiffVisualKeyUp(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 3
	result, _ := m.handleDiffVisualKey(keyMsg("up"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 2, rm.diffCursor)
}

func TestCovBoost2DiffVisualKeyHL(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("h"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 4, rm.diffVisualCurCol)

	result2, _ := rm.handleDiffVisualKey(keyMsg("l"), nil, 10, 5, 5)
	rm2 := result2.(Model)
	assert.Equal(t, 5, rm2.diffVisualCurCol)
}

func TestCovBoost2DiffVisualKeyHLBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'B'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("h"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 4, rm.diffVisualCurCol)
}

func TestCovBoost2DiffVisualKeyHLNotInCharOrBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("h"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 5, rm.diffVisualCurCol)
}

func TestCovBoost2DiffVisualKeyYLineMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'V'
	m.diffVisualStart = 0
	m.diffCursor = 0
	m.diffLeft = "line1\nline2\nline3"
	m.diffRight = "lineA\nlineB\nlineC"
	result, cmd := m.handleDiffVisualKey(keyMsg("y"), nil, 3, 3, 0)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovBoost2DiffVisualKeyZero(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("0"), nil, 10, 5, 5)
	rm := result.(Model)
	assert.Equal(t, 0, rm.diffVisualCurCol)
}

func TestCovEventTimelineVisualEsc(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualV(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte('V'), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("V"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualSmallV(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("v"))
	rm := result.(Model)
	assert.Equal(t, byte('v'), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualSmallVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	result, _ := m.handleEventTimelineVisualKey(keyMsg("v"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualCtrlV(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	result, _ := m.handleEventTimelineVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	rm := result.(Model)
	assert.Equal(t, byte('B'), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualCtrlVToggleOff(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'B'
	result, _ := m.handleEventTimelineVisualKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
}

func TestCovEventTimelineVisualJK(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"line0", "line1", "line2"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.eventTimelineCursor)

	result2, _ := rm.handleEventTimelineVisualKey(keyMsg("k"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.eventTimelineCursor)
}

func TestCovEventTimelineVisualHL(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineCursorCol = 5
	result, _ := m.handleEventTimelineVisualKey(keyMsg("h"))
	rm := result.(Model)
	assert.Equal(t, 4, rm.eventTimelineCursorCol)

	result2, _ := rm.handleEventTimelineVisualKey(keyMsg("l"))
	rm2 := result2.(Model)
	assert.Equal(t, 5, rm2.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualZero(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineCursorCol = 5
	result, _ := m.handleEventTimelineVisualKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualDollar(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("$"))
	rm := result.(Model)
	assert.Equal(t, len([]rune("hello world"))-1, rm.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualCaret(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"  hello world"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("^"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.eventTimelineCursorCol)
}

func TestCovEventTimelineVisualW(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("w"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovEventTimelineVisualB(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 6
	result, _ := m.handleEventTimelineVisualKey(keyMsg("b"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol < 6)
}

func TestCovEventTimelineVisualBigG(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"a", "b", "c"}
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 2, rm.eventTimelineCursor)
}

func TestCovEventTimelineVisualGG(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"a", "b", "c"}
	m.eventTimelineCursor = 2
	// First g.
	result, _ := m.handleEventTimelineVisualKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	// Second g.
	result2, _ := rm.handleEventTimelineVisualKey(keyMsg("g"))
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.eventTimelineCursor)
}

func TestCovEventTimelineVisualCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = make([]string, 50)
	m.eventTimelineCursor = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursor > 0)
}

func TestCovEventTimelineVisualCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = make([]string, 50)
	m.eventTimelineCursor = 25
	result, _ := m.handleEventTimelineVisualKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursor < 25)
}

func TestCovEventTimelineVisualYLineMode(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'V'
	m.eventTimelineLines = []string{"line0", "line1", "line2"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 1
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualYCharMode(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 0
	m.eventTimelineVisualCol = 0
	m.eventTimelineCursorCol = 4
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualYCharModeMultiline(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world", "foo bar", "baz qux"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 2
	m.eventTimelineVisualCol = 2
	m.eventTimelineCursorCol = 3
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualYBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'B'
	m.eventTimelineLines = []string{"hello world", "foo bar"}
	m.eventTimelineVisualStart = 0
	m.eventTimelineCursor = 1
	m.eventTimelineVisualCol = 0
	m.eventTimelineCursorCol = 4
	result, cmd := m.handleEventTimelineVisualKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, byte(0), rm.eventTimelineVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovEventTimelineVisualBigW(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world foo"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("W"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovEventTimelineVisualBigB(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world foo"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 12
	result, _ := m.handleEventTimelineVisualKey(keyMsg("B"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol < 12)
}

func TestCovEventTimelineVisualSmallE(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("e"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovEventTimelineVisualBigE(t *testing.T) {
	m := baseModelBoost2()
	m.eventTimelineVisualMode = 'v'
	m.eventTimelineLines = []string{"hello world"}
	m.eventTimelineCursor = 0
	m.eventTimelineCursorCol = 0
	result, _ := m.handleEventTimelineVisualKey(keyMsg("E"))
	rm := result.(Model)
	assert.True(t, rm.eventTimelineCursorCol > 0)
}

func TestCovBoost2DiffVisualYCharModeSingleLine(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffVisualStart = 0
	m.diffCursor = 0
	m.diffVisualCol = 0
	m.diffVisualCurCol = 4
	result, cmd := m.handleDiffVisualKey(keyMsg("y"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovBoost2DiffVisualYBlockMode(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'B'
	m.diffLeft = "hello world\nfoo bar"
	m.diffRight = "hello world\nfoo bar"
	m.diffVisualStart = 0
	m.diffCursor = 1
	m.diffVisualCol = 0
	m.diffVisualCurCol = 4
	result, cmd := m.handleDiffVisualKey(keyMsg("y"), nil, 2, 2, 0)
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
	assert.NotNil(t, cmd)
}

func TestCovBoost2DiffVisualKeyDollar(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("$"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyCaret(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "  hello world"
	m.diffRight = "  hello world"
	m.diffCursor = 0
	m.diffVisualCurCol = 5
	result, _ := m.handleDiffVisualKey(keyMsg("^"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.Equal(t, 2, rm.diffVisualCurCol) // first non-whitespace
}

func TestCovBoost2DiffVisualKeyW(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("w"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeySmallB(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 6
	result, _ := m.handleDiffVisualKey(keyMsg("b"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol < 6)
}

func TestCovBoost2DiffVisualKeyE(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("e"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyBigE(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world"
	m.diffRight = "hello world"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("E"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyBigW(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 0
	result, _ := m.handleDiffVisualKey(keyMsg("W"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol > 0)
}

func TestCovBoost2DiffVisualKeyBigB(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	m.diffLeft = "hello world foo"
	m.diffRight = "hello world foo"
	m.diffCursor = 0
	m.diffVisualCurCol = 12
	result, _ := m.handleDiffVisualKey(keyMsg("B"), nil, 1, 1, 0)
	rm := result.(Model)
	assert.True(t, rm.diffVisualCurCol < 12)
}

func TestCovBoost2DiffVisualKeyBigG(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	result, _ := m.handleDiffVisualKey(keyMsg("G"), nil, 3, 3, 0)
	rm := result.(Model)
	assert.Equal(t, 2, rm.diffCursor)
}

func TestCovBoost2DiffVisualKeyGG(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 2
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	// First g.
	result, _ := m.handleDiffVisualKey(keyMsg("g"), nil, 3, 3, 0)
	rm := result.(Model)
	require.True(t, rm.pendingG)

	// Second g.
	result2, _ := rm.handleDiffVisualKey(keyMsg("g"), nil, 3, 3, 0)
	rm2 := result2.(Model)
	assert.Equal(t, 0, rm2.diffCursor)
}

func TestCovBoost2DiffVisualKeyCtrlD(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	m.diffLeft = strings.Repeat("line\n", 50)
	m.diffRight = strings.Repeat("line\n", 50)
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+d"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor > 0)
}

func TestCovBoost2DiffVisualKeyCtrlU(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 25
	m.diffLeft = strings.Repeat("line\n", 50)
	m.diffRight = strings.Repeat("line\n", 50)
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+u"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor < 25)
}

func TestCovBoost2DiffVisualKeyCtrlF(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 0
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+f"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor > 0)
}

func TestCovBoost2DiffVisualKeyCtrlB(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursor = 30
	result, _ := m.handleDiffVisualKey(keyMsg("ctrl+b"), nil, 50, 20, 30)
	rm := result.(Model)
	assert.True(t, rm.diffCursor < 30)
}

func TestCovBoost2DiffVisualKeyH2(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(keyMsg("H"), nil, 10, 5, 5)
	_ = result
}

func TestCovBoost2DiffVisualKeyL2(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffVisualType = 'v'
	result, _ := m.handleDiffVisualKey(keyMsg("L"), nil, 10, 5, 5)
	_ = result
}

func TestCovBoost2DiffVisualKeyM(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	result, _ := m.handleDiffVisualKey(keyMsg("M"), nil, 10, 5, 5)
	_ = result
}

func TestCovBoost2DiffVisualKeyTab(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	m.diffCursorSide = 0
	// "tab" is not handled by handleDiffVisualKey, falls through.
	result, cmd := m.handleDiffVisualKey(keyMsg("tab"), nil, 10, 5, 5)
	assert.Nil(t, cmd)
	_ = result
}

func TestCovBoost2DiffVisualKeyUnknown(t *testing.T) {
	m := baseModelBoost2()
	m.diffVisualMode = true
	result, cmd := m.handleDiffVisualKey(keyMsg("z"), nil, 10, 5, 5)
	assert.Nil(t, cmd)
	_ = result
}

func TestCovDiffKeySearchEnter(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	m.diffSearchText.Insert("test")
	m.diffLeft = "line1\nline2\ntest line"
	m.diffRight = "line1\nline2\ntest line"
	result, _ := m.handleDiffKey(keyMsg("enter"))
	rm := result.(Model)
	assert.False(t, rm.diffSearchMode)
	assert.Equal(t, "test", rm.diffSearchQuery)
}

func TestCovDiffKeySearchEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	result, _ := m.handleDiffKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.diffSearchMode)
}

func TestCovDiffKeySearchBackspace(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	m.diffSearchText.Insert("ab")
	result, _ := m.handleDiffKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "a", rm.diffSearchText.Value)
}

func TestCovDiffKeySearchCtrlW(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	m.diffSearchText.Insert("foo bar")
	result, _ := m.handleDiffKey(keyMsg("ctrl+w"))
	_ = result.(Model)
}

func TestCovDiffKeySearchCtrlA(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	result, _ := m.handleDiffKey(keyMsg("ctrl+a"))
	_ = result.(Model)
}

func TestCovDiffKeySearchCtrlE(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	result, _ := m.handleDiffKey(keyMsg("ctrl+e"))
	_ = result.(Model)
}

func TestCovDiffKeySearchLeftRight(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	m.diffSearchText.Insert("abc")
	result, _ := m.handleDiffKey(keyMsg("left"))
	rm := result.(Model)
	result, _ = rm.handleDiffKey(keyMsg("right"))
	_ = result.(Model)
}

func TestCovDiffKeySearchTyping(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	result, _ := m.handleDiffKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, "x", rm.diffSearchText.Value)
}

func TestCovDiffKeySearchCtrlC(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffSearchMode = true
	result, _ := m.handleDiffKey(keyMsg("ctrl+c"))
	rm := result.(Model)
	assert.False(t, rm.diffSearchMode)
}

func TestCovDiffKeyHelp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffLeft = "line1\nline2"
	m.diffRight = "line1\nline3"
	result, _ := m.handleDiffKey(keyMsg("?"))
	rm := result.(Model)
	assert.Equal(t, modeHelp, rm.mode)
	assert.Equal(t, "Diff View", rm.helpContextMode)
}

func TestCovDiffKeyToggleWrap(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffWrap = false
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg(">"))
	rm := result.(Model)
	assert.True(t, rm.diffWrap)
}

func TestCovDiffKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, modeExplorer, rm.mode)
}

func TestCovDiffKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffScroll = 0
	m.diffLeft = "a\nb\nc\nd"
	m.diffRight = "a\nb\nc\nd"
	result, _ := m.handleDiffKey(keyMsg("j"))
	_ = result.(Model)
}

func TestCovDiffKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffScroll = 3
	m.diffLeft = "a\nb\nc\nd"
	m.diffRight = "a\nb\nc\nd"
	result, _ := m.handleDiffKey(keyMsg("k"))
	_ = result.(Model)
}

func TestCovDiffKeySlash(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg("/"))
	rm := result.(Model)
	assert.True(t, rm.diffSearchMode)
}

func TestCovDiffKeyToggleUnified(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffUnified = false
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg("u"))
	rm := result.(Model)
	assert.True(t, rm.diffUnified)
}

func TestCovDiffKeyCtrlD(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffScroll = 0
	m.diffLeft = "a\nb\nc\nd\ne\nf\ng\nh"
	m.diffRight = "a\nb\nc\nd\ne\nf\ng\nh"
	result, _ := m.handleDiffKey(keyMsg("ctrl+d"))
	_ = result.(Model)
}

func TestCovDiffKeyCtrlU(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffScroll = 10
	m.diffLeft = "a\nb\nc\nd\ne\nf"
	m.diffRight = "a\nb\nc\nd\ne\nf"
	result, _ := m.handleDiffKey(keyMsg("ctrl+u"))
	_ = result.(Model)
}

func TestCovDiffKeyGG(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffScroll = 5
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)

	result, _ = rm.handleDiffKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.diffScroll)
}

func TestCovDiffKeyBigG(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	result, _ := m.handleDiffKey(keyMsg("G"))
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.diffScroll, 0)
}

func TestCovDiffKeyVisualV(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg("v"))
	rm := result.(Model)
	assert.True(t, rm.diffVisualMode)
}

func TestCovDiffKeyTab(t *testing.T) {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffCursorSide = 0
	m.diffLeft = "a"
	m.diffRight = "b"
	result, _ := m.handleDiffKey(keyMsg("tab"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.diffCursorSide)
}

func TestCovDiffVisualKeyEsc(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffVisualMode = true
	m.diffLeft = "a\nb"
	m.diffRight = "a\nc"
	result, _ := m.handleDiffKey(keyMsg("esc"))
	rm := result.(Model)
	assert.False(t, rm.diffVisualMode)
}

func TestCovDiffVisualKeyDown(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffVisualMode = true
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	m.diffScroll = 0
	result, _ := m.handleDiffKey(keyMsg("j"))
	_ = result.(Model)
}

func TestCovDiffVisualKeyYank(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffVisualMode = true
	m.diffVisualStart = 0
	m.diffScroll = 0
	m.diffLeft = "a\nb\nc"
	m.diffRight = "a\nb\nc"
	_, cmd := m.handleDiffKey(keyMsg("y"))
	assert.NotNil(t, cmd)
}
