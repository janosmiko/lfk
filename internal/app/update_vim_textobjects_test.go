package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- innerWordRange ---

func TestInnerWordRange(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		col       int
		wantStart int
		wantEnd   int
	}{
		{"empty", "", 0, -1, -1},
		{"cursor on first char", "hello world", 0, 0, 4},
		{"cursor mid-word", "hello world", 2, 0, 4},
		{"cursor on space between words", "hello world", 5, 5, 5},
		{"cursor on second word", "hello world", 6, 6, 10},
		{"cursor on punct boundary lumped together", "foo, bar", 3, 3, 4},
		{"cursor on word with leading punct grouped boundary", "foo:bar", 3, 3, 3},
		{"cursor past end clamps", "hi", 10, 0, 1},
		{"cursor on hyphen treated as boundary", "a-b", 1, 1, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := innerWordRange(tt.line, tt.col)
			assert.Equal(t, tt.wantStart, start, "start")
			assert.Equal(t, tt.wantEnd, end, "end")
		})
	}
}

// --- aroundWordRange ---

func TestAroundWordRange(t *testing.T) {
	tests := []struct {
		name      string
		line      string
		col       int
		wantStart int
		wantEnd   int
	}{
		{"empty", "", 0, -1, -1},
		{"word with trailing space", "hello world", 0, 0, 5},
		{"word with no trailing, leading space included", "hello world", 6, 5, 10},
		{"trailing-only at end of line", "hi  ", 0, 0, 3},
		{"cursor on whitespace, swallows next word", "  hello", 0, 0, 6},
		{"cursor at last char, leading whitespace included", "  hello", 6, 0, 6},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end := aroundWordRange(tt.line, tt.col)
			assert.Equal(t, tt.wantStart, start, "start")
			assert.Equal(t, tt.wantEnd, end, "end")
		})
	}
}

// --- innerWORDRange / aroundWORDRange ---

func TestInnerWORDRange(t *testing.T) {
	// WORD treats only space/tab as boundaries, so punctuation runs with the word.
	start, end := innerWORDRange("foo-bar baz", 0)
	assert.Equal(t, 0, start)
	assert.Equal(t, 6, end, "WORD spans across hyphen until whitespace")
}

func TestAroundWORDRange(t *testing.T) {
	start, end := aroundWORDRange("foo-bar baz", 0)
	assert.Equal(t, 0, start)
	assert.Equal(t, 7, end, "around WORD includes trailing whitespace")
}

// --- textObjectRange dispatcher ---

func TestTextObjectRangeDispatch(t *testing.T) {
	line := "alpha beta"
	cases := []struct {
		op        byte
		motion    string
		col       int
		wantStart int
		wantEnd   int
		wantOK    bool
	}{
		{'i', "w", 0, 0, 4, true},
		{'a', "w", 0, 0, 5, true},
		{'i', "W", 0, 0, 4, true},
		{'a', "W", 0, 0, 5, true},
		{'i', "x", 0, 0, 0, false}, // unsupported motion
		{'z', "w", 0, 0, 0, false}, // unsupported operator
	}
	for _, c := range cases {
		s, e, ok := textObjectRange(line, c.col, c.op, c.motion)
		assert.Equal(t, c.wantOK, ok, "ok for op=%c motion=%s", c.op, c.motion)
		if ok {
			assert.Equal(t, c.wantStart, s, "start for op=%c motion=%s", c.op, c.motion)
			assert.Equal(t, c.wantEnd, e, "end for op=%c motion=%s", c.op, c.motion)
		}
	}
}

// --- log viewer end-to-end ---

func TestLogVisualViwSelectsInnerWord(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logLines = []string{"alpha beta gamma"}
	m.logCursor = 0
	m.logVisualCurCol = 7 // on 'e' of "beta"
	m.logVisualMode = true
	m.logVisualType = 'v'
	m.logVisualStart = 0
	m.logVisualCol = 7

	// `i`
	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	m1 := r1.(Model)
	assert.Equal(t, byte('i'), m1.pendingTextObject)

	// `w` resolves to inner word
	r2, _ := m1.handleLogVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, byte(0), m2.pendingTextObject, "operator cleared after resolution")
	assert.Equal(t, rune('v'), m2.logVisualType, "switched to char-wise visual")
	assert.Equal(t, 6, m2.logVisualCol, "selection start at 'b' of beta")
	assert.Equal(t, 9, m2.logVisualCurCol, "selection end at 'a' of beta")
}

func TestLogVisualVawSelectsAroundWord(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logLines = []string{"alpha beta gamma"}
	m.logCursor = 0
	m.logVisualCurCol = 7
	m.logVisualMode = true
	m.logVisualType = 'v'
	m.logVisualStart = 0
	m.logVisualCol = 7

	r1, _ := m.handleLogVisualKey(keyMsg("a"))
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, 6, m2.logVisualCol)
	assert.Equal(t, 10, m2.logVisualCurCol, "includes trailing space")
}

func TestLogVisualPendingClearedOnUnknownKey(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logLines = []string{"alpha beta"}
	m.logCursor = 0
	m.logVisualMode = true
	m.logVisualType = 'v'

	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	assert.Equal(t, byte('i'), r1.(Model).pendingTextObject)
	// `j` cancels operator and falls through to normal motion.
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("j"))
	assert.Equal(t, byte(0), r2.(Model).pendingTextObject)
}

func TestLogVisualEscClearsPendingOperator(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logLines = []string{"alpha"}
	m.logVisualMode = true
	m.pendingTextObject = 'i'

	r, _ := m.handleLogVisualKey(keyMsg("esc"))
	rm := r.(Model)
	assert.Equal(t, byte(0), rm.pendingTextObject)
	assert.False(t, rm.logVisualMode)
}

// --- describe viewer ---

func TestDescribeVisualViwSelectsInnerWord(t *testing.T) {
	m := Model{
		mode:               modeDescribe,
		describeContent:    "alpha beta gamma",
		describeCursor:     0,
		describeCursorCol:  7, // on 'e' of "beta"
		describeVisualMode: 'v',
		tabs:               []TabState{{}},
		width:              80,
		height:             40,
	}

	r1, _ := m.handleDescribeVisualKey(keyMsg("i"))
	assert.Equal(t, byte('i'), r1.(Model).pendingTextObject)

	r2, _ := r1.(Model).handleDescribeVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, byte(0), m2.pendingTextObject)
	assert.Equal(t, byte('v'), m2.describeVisualMode)
	assert.Equal(t, 6, m2.describeVisualCol)
	assert.Equal(t, 9, m2.describeCursorCol)
}

// --- diff viewer ---

func TestDiffVisualViwSetsPendingThenResolves(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffVisualMode = true
	m.diffVisualType = 'v'

	r1, _ := m.handleDiffVisualKey(keyMsg("i"), nil, 1, 5, 0)
	assert.Equal(t, byte('i'), r1.(Model).pendingTextObject)

	// `j` cancels the operator without crashing on missing diff content.
	r2, _ := r1.(Model).handleDiffVisualKey(keyMsg("j"), nil, 1, 5, 0)
	assert.Equal(t, byte(0), r2.(Model).pendingTextObject)
}

// --- events viewer ---

func TestEventsVisualViwSelectsInnerWord(t *testing.T) {
	m := Model{
		eventTimelineLines:       []string{"alpha beta gamma"},
		eventTimelineCursor:      0,
		eventTimelineCursorCol:   7,
		eventTimelineVisualMode:  'v',
		eventTimelineVisualStart: 0,
		eventTimelineVisualCol:   7,
		tabs:                     []TabState{{}},
		width:                    80,
		height:                   40,
	}

	r1, _ := m.handleEventTimelineVisualKey(keyMsg("i"))
	assert.Equal(t, byte('i'), r1.(Model).pendingTextObject)

	r2, _ := r1.(Model).handleEventTimelineVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, byte(0), m2.pendingTextObject)
	assert.Equal(t, byte('v'), m2.eventTimelineVisualMode)
	assert.Equal(t, 6, m2.eventTimelineVisualCol)
	assert.Equal(t, 9, m2.eventTimelineCursorCol)
}

// --- pendingG cleared on text-object resolution ---

func TestLogVisualTextObjectClearsStalePendingG(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logLines = []string{"alpha beta"}
	m.logVisualMode = true
	m.logVisualType = 'v'
	m.pendingG = true
	m.pendingTextObject = 'i'

	r, _ := m.handleLogVisualKey(keyMsg("w"))
	rm := r.(Model)
	assert.False(t, rm.pendingG, "stale pendingG must not survive a text-object resolve")
}

// --- yaml viewer (covers fold-prefix offset) ---

func TestYAMLVisualViWRespectsFoldPrefix(t *testing.T) {
	m := baseModelNav()
	m.mode = modeYAML
	// YAML visible lines include a 2-char fold prefix, so a word at original
	// col 0 lives at visible col 2.
	m.yamlContent = "foo: bar\n"
	m.yamlCursor = 0
	m.yamlVisualCurCol = yamlFoldPrefixLen + 5 // on 'b' of "bar"
	m.yamlVisualMode = true
	m.yamlVisualType = 'v'

	r1, _ := m.handleYAMLVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleYAMLVisualKey(keyMsg("W"))
	m2 := r2.(Model)
	assert.Equal(t, rune('v'), m2.yamlVisualType)
	assert.GreaterOrEqual(t, m2.yamlVisualCol, yamlFoldPrefixLen, "selection start clamped past fold prefix")
	assert.GreaterOrEqual(t, m2.yamlVisualCurCol, m2.yamlVisualCol)
}
