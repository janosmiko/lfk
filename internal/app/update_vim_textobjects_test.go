package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
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
	m.logView.lines = []string{"alpha beta gamma"}
	m.logView.cursor = 0
	m.logView.visualCurCol = 7 // on 'e' of "beta"
	m.logView.visualMode = true
	m.logView.visualType = 'v'
	m.logView.visualStart = 0
	m.logView.visualCol = 7

	// `i`
	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	m1 := r1.(Model)
	assert.Equal(t, byte('i'), m1.pendingTextObject)

	// `w` resolves to inner word
	r2, _ := m1.handleLogVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, byte(0), m2.pendingTextObject, "operator cleared after resolution")
	assert.Equal(t, rune('v'), m2.logView.visualType, "switched to char-wise visual")
	assert.Equal(t, 6, m2.logView.visualCol, "selection start at 'b' of beta")
	assert.Equal(t, 9, m2.logView.visualCurCol, "selection end at 'a' of beta")
}

func TestLogVisualVawSelectsAroundWord(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.lines = []string{"alpha beta gamma"}
	m.logView.cursor = 0
	m.logView.visualCurCol = 7
	m.logView.visualMode = true
	m.logView.visualType = 'v'
	m.logView.visualStart = 0
	m.logView.visualCol = 7

	r1, _ := m.handleLogVisualKey(keyMsg("a"))
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, 6, m2.logView.visualCol)
	assert.Equal(t, 10, m2.logView.visualCurCol, "includes trailing space")
}

func TestLogVisualPendingClearedOnUnknownKey(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.lines = []string{"alpha beta"}
	m.logView.cursor = 0
	m.logView.visualMode = true
	m.logView.visualType = 'v'

	r1, _ := m.handleLogVisualKey(keyMsg("i"))
	assert.Equal(t, byte('i'), r1.(Model).pendingTextObject)
	// `j` cancels operator and falls through to normal motion.
	r2, _ := r1.(Model).handleLogVisualKey(keyMsg("j"))
	assert.Equal(t, byte(0), r2.(Model).pendingTextObject)
}

func TestLogVisualEscClearsPendingOperator(t *testing.T) {
	m := baseModelNav()
	m.mode = modeLogs
	m.logView.lines = []string{"alpha"}
	m.logView.visualMode = true
	m.pendingTextObject = 'i'

	r, _ := m.handleLogVisualKey(keyMsg("esc"))
	rm := r.(Model)
	assert.Equal(t, byte(0), rm.pendingTextObject)
	assert.False(t, rm.logView.visualMode)
}

// --- describe viewer ---

func TestDescribeVisualViwSelectsInnerWord(t *testing.T) {
	m := Model{
		mode: modeDescribe,
		describeView: describeViewState{
			content:    "alpha beta gamma",
			cursor:     0,
			cursorCol:  7, // on 'e' of "beta"
			visualMode: 'v',
		},
		tabs:   []TabState{{}},
		width:  80,
		height: 40,
	}

	r1, _ := m.handleDescribeVisualKey(keyMsg("i"))
	assert.Equal(t, byte('i'), r1.(Model).pendingTextObject)

	r2, _ := r1.(Model).handleDescribeVisualKey(keyMsg("w"))
	m2 := r2.(Model)
	assert.Equal(t, byte(0), m2.pendingTextObject)
	assert.Equal(t, byte('v'), m2.describeView.visualMode)
	assert.Equal(t, 6, m2.describeView.visualCol)
	assert.Equal(t, 9, m2.describeView.cursorCol)
}

// --- diff viewer ---

func TestDiffVisualViwSetsPendingThenResolves(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDiff
	m.diffView.visualMode = true
	m.diffView.visualType = 'v'

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
	m.logView.lines = []string{"alpha beta"}
	m.logView.visualMode = true
	m.logView.visualType = 'v'
	m.pendingG = true
	m.pendingTextObject = 'i'

	r, _ := m.handleLogVisualKey(keyMsg("w"))
	rm := r.(Model)
	assert.False(t, rm.pendingG, "stale pendingG must not survive a text-object resolve")
}

// --- loadTab clears pendingTextObject ---

// Regression for the bug where a half-typed text-object operator (`i`/`a`) on
// one tab survived a tab switch and was applied to the first key pressed in
// the next tab's visual mode. pendingTextObject lives on Model, so it had to
// be reset alongside the other transient state in loadTab.
func TestLoadTabClearsPendingTextObject(t *testing.T) {
	m := Model{
		tabs: []TabState{
			{nav: model.NavigationState{Context: "kctx"}},
			{nav: model.NavigationState{Context: "kctx"}},
		},
		activeTab:         0,
		pendingTextObject: 'i',
	}

	m.saveCurrentTab()
	_ = m.loadTab(1)

	assert.Equal(t, byte(0), m.pendingTextObject,
		"loadTab must clear pendingTextObject so a half-typed operator on tab 0 does not leak into tab 1's visual mode")
}

// --- yaml viewer (covers fold-prefix offset) ---

func TestYAMLVisualViWRespectsFoldPrefix(t *testing.T) {
	m := baseModelNav()
	m.mode = modeYAML
	// YAML visible lines include a 2-char fold prefix, so a word at original
	// col 0 lives at visible col 2.
	m.yamlView.content = "foo: bar\n"
	m.yamlView.cursor = 0
	m.yamlView.visualCurCol = yamlFoldPrefixLen + 5 // on 'b' of "bar"
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'

	r1, _ := m.handleYAMLVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleYAMLVisualKey(keyMsg("W"))
	m2 := r2.(Model)
	assert.Equal(t, rune('v'), m2.yamlView.visualType)
	assert.GreaterOrEqual(t, m2.yamlView.visualCol, yamlFoldPrefixLen, "selection start clamped past fold prefix")
	assert.GreaterOrEqual(t, m2.yamlView.visualCurCol, m2.yamlView.visualCol)
}

// Regression for the degenerate clamp: when the cursor sits inside the
// 2-char fold-prefix gutter and the resolved range is entirely inside that
// gutter, the prior `start = max(start, foldPrefix); end = max(end, start)`
// silently collapsed the selection onto col yamlFoldPrefixLen and flipped
// the type to 'v', with no visible change to confirm the operation.
// applyYAMLTextObject now bails out so the existing selection state is
// preserved.
func TestYAMLVisualViwInsideFoldPrefixPreservesSelection(t *testing.T) {
	m := baseModelNav()
	m.mode = modeYAML
	m.yamlView.content = "foo: bar\n"
	// A non-empty sections slice causes buildVisibleLines to prepend the
	// 2-char fold-prefix gutter — without it, the gutter doesn't exist and
	// this regression isn't reachable.
	m.yamlView.sections = []yamlSection{
		{key: "root", startLine: 0, endLine: 0},
	}
	m.yamlView.collapsed = make(map[string]bool)
	m.yamlView.cursor = 0
	// Cursor in the gutter; the inner-word range resolves to (0, 1) —
	// entirely inside the fold prefix, since the visible line is "  foo: bar".
	m.yamlView.visualCurCol = 0
	m.yamlView.visualCol = 0
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'V' // sentinel to detect a stray flip to 'v'

	r1, _ := m.handleYAMLVisualKey(keyMsg("i"))
	r2, _ := r1.(Model).handleYAMLVisualKey(keyMsg("w"))
	m2 := r2.(Model)

	assert.Equal(t, rune('V'), m2.yamlView.visualType,
		"selection type must not flip to 'v' when the resolved range lies entirely in the fold prefix")
	assert.Equal(t, 0, m2.yamlView.visualCol,
		"selection start must remain untouched when the range is dropped")
	assert.Equal(t, 0, m2.yamlView.visualCurCol,
		"selection cursor must remain untouched when the range is dropped")
}

// --- operator-pending entry clears stale digit prefix ---

// Regression for the bug where a digit prefix typed before visual entry
// (e.g. `5` followed by `v`) survived in the per-viewer LineInput buffer
// because yaml/describe visual entry didn't clear it. The PR's new `i`/`a`
// operator-pending path was the most visible exposure: a stale "5" would
// silently leak into the next counted command after the visual exit. The
// fix clears the buffer when entering operator-pending so the new code
// path is unambiguous regardless of what the visual-entry handlers do.

func TestYAMLVisualOperatorPendingClearsLineInput(t *testing.T) {
	m := baseModelNav()
	m.mode = modeYAML
	m.yamlView.content = "foo: bar\n"
	m.yamlView.visualMode = true
	m.yamlView.visualType = 'v'
	m.yamlView.visualCurCol = yamlFoldPrefixLen
	m.yamlView.lineInput = "5" // stale digit from before visual entry

	r, _ := m.handleYAMLVisualKey(keyMsg("i"))
	rm := r.(Model)

	assert.Equal(t, byte('i'), rm.pendingTextObject)
	assert.Equal(t, "", rm.yamlView.lineInput,
		"entering operator-pending must clear the count buffer so a stale digit can't leak into the next counted command")
}

func TestDescribeVisualOperatorPendingClearsLineInput(t *testing.T) {
	m := baseModelNav()
	m.mode = modeDescribe
	m.describeView.content = "alpha beta gamma"
	m.describeView.visualMode = 'v'
	m.describeView.lineInput = "5" // stale digit from before visual entry

	r, _ := m.handleDescribeVisualKey(keyMsg("a"))
	rm := r.(Model)

	assert.Equal(t, byte('a'), rm.pendingTextObject)
	assert.Equal(t, "", rm.describeView.lineInput,
		"entering operator-pending must clear the count buffer so a stale digit can't leak into the next counted command")
}
