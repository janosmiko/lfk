package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// `123j` / `123k` reuse the same digit accumulator that powers `123y` and
// `123G`. The buffer must be consumed by the motion (so digits don't leak
// into the next command), the cursor must move by the requested count, and
// the count must clamp to the available range rather than walking off the
// end.

func TestYAMLNormalCountPrefixJumpsDown(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6",
		yamlCollapsed: map[string]bool{},
		yamlCursor:    0,
		yamlLineInput: "3",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 3, rm.yamlCursor)
	assert.Empty(t, rm.yamlLineInput, "digit buffer must be consumed by the motion")
}

func TestYAMLNormalCountPrefixJumpsUp(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   "a: 1\nb: 2\nc: 3\nd: 4\ne: 5\nf: 6",
		yamlCollapsed: map[string]bool{},
		yamlCursor:    5,
		yamlLineInput: "3",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 2, rm.yamlCursor)
	assert.Empty(t, rm.yamlLineInput)
}

func TestYAMLNormalCountClampsAtBottom(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   "a: 1\nb: 2\nc: 3",
		yamlCollapsed: map[string]bool{},
		yamlCursor:    1,
		yamlLineInput: "100",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 2, rm.yamlCursor, "count must clamp to last visible line")
}

func TestYAMLNormalCountClampsAtTop(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   "a: 1\nb: 2\nc: 3",
		yamlCollapsed: map[string]bool{},
		yamlCursor:    1,
		yamlLineInput: "100",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.yamlCursor)
}

func TestDescribeNormalCountPrefixJumpsDown(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 0
	m.describeLineInput = "4"
	ret, _ := m.handleDescribeKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 4, rm.describeCursor)
	assert.Empty(t, rm.describeLineInput)
}

func TestDescribeNormalCountPrefixJumpsUp(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 8
	m.describeLineInput = "5"
	ret, _ := m.handleDescribeKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 3, rm.describeCursor)
	assert.Empty(t, rm.describeLineInput)
}

func TestDescribeNormalCountClampsAtBottom(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 8
	m.describeLineInput = "100"
	ret, _ := m.handleDescribeKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 9, rm.describeCursor, "10-line fixture clamps to index 9")
}

func TestDiffNormalCountPrefixJumpsDown(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a\nb\nc\nd\ne\nf", diffRight: "a\nb\nc\nd\ne\nf",
		diffLeftName: "before", diffRightName: "after",
		diffCursor:    0,
		diffLineInput: "3",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 3, rm.diffCursor)
	assert.Empty(t, rm.diffLineInput)
}

func TestDiffNormalCountPrefixJumpsUp(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a\nb\nc\nd\ne\nf", diffRight: "a\nb\nc\nd\ne\nf",
		diffLeftName: "before", diffRightName: "after",
		diffCursor:    5,
		diffLineInput: "4",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 1, rm.diffCursor)
	assert.Empty(t, rm.diffLineInput)
}

func TestDiffNormalCountClampsAtTop(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a\nb\nc", diffRight: "a\nb\nc",
		diffLeftName: "before", diffRightName: "after",
		diffCursor:    1,
		diffLineInput: "100",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.diffCursor)
}

func TestLogsNormalCountPrefixJumpsDown(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     []string{"a", "b", "c", "d", "e", "f"},
		logCursor:    0,
		logFollow:    true,
		logLineInput: "3",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 3, rm.logCursor)
	assert.Empty(t, rm.logLineInput)
	assert.False(t, rm.logFollow, "any j press disables follow mode")
}

func TestLogsNormalCountPrefixJumpsUp(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     []string{"a", "b", "c", "d", "e", "f"},
		logCursor:    5,
		logLineInput: "4",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 1, rm.logCursor)
	assert.Empty(t, rm.logLineInput)
}

func TestLogsNormalCountClampsAtTop(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     []string{"a", "b", "c"},
		logCursor:    1,
		logLineInput: "100",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.logCursor)
}

func TestLogsNormalCountClampsAtBottom(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     []string{"a", "b", "c"},
		logCursor:    1,
		logLineInput: "100",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 2, rm.logCursor)
}

// Empty log buffer must not let the cursor go negative when a count motion
// fires before any lines arrive — the `max(len-1, 0)` guard in handleLogKeyJ
// keeps cursor pinned at 0.
func TestLogsNormalCountOnEmptyBufferStaysAtZero(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     nil,
		logCursor:    0,
		logLineInput: "5",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.logCursor)
	assert.Empty(t, rm.logLineInput)
}

func TestEventTimelineCountPrefixJumpsDown(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"e0", "e1", "e2", "e3", "e4"},
		eventTimelineCursor:    0,
		eventTimelineLineInput: "3",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("j"))
	rm := ret.(Model)
	assert.Equal(t, 3, rm.eventTimelineCursor)
	assert.Empty(t, rm.eventTimelineLineInput)
}

func TestEventTimelineCountPrefixJumpsUp(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"e0", "e1", "e2", "e3", "e4"},
		eventTimelineCursor:    4,
		eventTimelineLineInput: "3",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 1, rm.eventTimelineCursor)
	assert.Empty(t, rm.eventTimelineLineInput)
}

func TestEventTimelineCountClampsAtTop(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"e0", "e1", "e2"},
		eventTimelineCursor:    1,
		eventTimelineLineInput: "100",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("k"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.eventTimelineCursor)
}

// Plain `j` / `k` (no digits) must keep their single-step behaviour so users
// who never type counts notice no change.
func TestPlainJKStillMovesByOne(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 4
	ret, _ := m.handleDescribeKey(keyMsg("j"))
	assert.Equal(t, 5, ret.(Model).describeCursor)

	m = baseModelDescribe()
	m.describeCursor = 4
	ret, _ = m.handleDescribeKey(keyMsg("k"))
	assert.Equal(t, 3, ret.(Model).describeCursor)
}

// --- Column motion: h/l with count ---
//
// `Nh` / `Nl` shift the cursor column by N runes; the buffer must be consumed
// and `h` must clamp at column 0 rather than walking negative.

func TestDescribeCountPrefixColumnRight(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursorCol = 0
	m.describeLineInput = "5"
	ret, _ := m.handleDescribeKey(keyMsg("l"))
	rm := ret.(Model)
	assert.Equal(t, 5, rm.describeCursorCol)
	assert.Empty(t, rm.describeLineInput)
}

func TestDescribeCountPrefixColumnLeftClampsAtZero(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursorCol = 3
	m.describeLineInput = "100"
	ret, _ := m.handleDescribeKey(keyMsg("h"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.describeCursorCol)
}

func TestLogsCountPrefixColumnRight(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:        []string{"hello world from logs"},
		logCursor:       0,
		logVisualCurCol: 0,
		logLineInput:    "6",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("l"))
	rm := ret.(Model)
	assert.Equal(t, 6, rm.logVisualCurCol)
	assert.Empty(t, rm.logLineInput)
}

// --- Word motion: w with count ---
//
// `Nw` advances by N word starts. Each step uses the existing single-step
// motion, so we only verify the count is consumed and the cursor lands past
// the third word boundary on the same line.

func TestDescribeCountPrefixWordForward(t *testing.T) {
	m := baseModelDescribe()
	// "line0" — only one word per line in the fixture; use a richer line.
	m.describeContent = "alpha beta gamma delta epsilon"
	m.describeCursor = 0
	m.describeCursorCol = 0
	m.describeLineInput = "3"
	ret, _ := m.handleDescribeKey(keyMsg("w"))
	rm := ret.(Model)
	// alpha(0) -> beta(6) -> gamma(11) -> delta(17): 3w lands at delta.
	assert.Equal(t, 17, rm.describeCursorCol)
	assert.Empty(t, rm.describeLineInput)
}

func TestLogsCountPrefixWordForward(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:        []string{"alpha beta gamma delta"},
		logCursor:       0,
		logVisualCurCol: 0,
		logLineInput:    "2",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("w"))
	rm := ret.(Model)
	// alpha(0) -> beta(6) -> gamma(11): 2w lands at gamma.
	assert.Equal(t, 11, rm.logVisualCurCol)
	assert.Empty(t, rm.logLineInput)
}

// --- Page motion: Ctrl+D / Ctrl+F with count ---
//
// `N<C-d>` and `N<C-f>` scale the half/full page step by N. We pick a viewport
// that gives a known step size and check the count multiplies it.

func TestDescribeCountPrefixHalfPageDown(t *testing.T) {
	m := baseModelDescribe()
	// describeContentHeight = max(height-4, 3) = 36 with height=40,
	// so half-page = 18. With a 10-line fixture and 100-line cursor max
	// the count clamps at the end; use a longer fixture to exercise the
	// multiplier without clamping.
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line"
	}
	m.describeContent = strings.Join(lines, "\n")
	m.describeCursor = 0
	m.describeLineInput = "2"
	ret, _ := m.handleDescribeKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	// 2 * (40-4)/2 = 36 lines.
	assert.Equal(t, 36, rm.describeCursor)
	assert.Empty(t, rm.describeLineInput)
}

// Regression: when content height is odd, `2<C-d>` must land at exactly
// `2 * (h/2)`, not `(2*h)/2`. With describeContentHeight = max(h-4, 3) = 5
// for height=9, single-step half-page is 2, so two presses should land at 4.
// The buggy `n*h/2` arithmetic would land at 5 (= 2*5/2), over-shooting by
// floor(n/2) lines.
func TestDescribeCountHalfPageOddHeightMatchesSingleSteps(t *testing.T) {
	m := baseModelDescribe()
	m.height = 9 // describeContentHeight = 5 (odd); single half-page = 2.
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line"
	}
	m.describeContent = strings.Join(lines, "\n")
	m.describeCursor = 0

	// Reference path: two single Ctrl+D presses.
	ref := m
	ret, _ := ref.handleDescribeKey(keyMsg("ctrl+d"))
	ref = ret.(Model)
	ret, _ = ref.handleDescribeKey(keyMsg("ctrl+d"))
	ref = ret.(Model)
	expected := ref.describeCursor

	// Counted path: one `2<C-d>`.
	m.describeLineInput = "2"
	ret, _ = m.handleDescribeKey(keyMsg("ctrl+d"))
	got := ret.(Model).describeCursor

	assert.Equal(t, expected, got, "2<C-d> must land where two single C-d presses do")
	assert.Equal(t, 4, got, "with content height 5, two half-pages of 2 = 4")
}

func TestLogsCountPrefixFullPageDown(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "x"
	}
	m := Model{
		width: 80, height: 40, mode: modeLogs,
		logLines:     lines,
		logCursor:    0,
		logFollow:    true,
		logLineInput: "2",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("ctrl+f"))
	rm := ret.(Model)
	// logContentHeight is positive; with count=2 cursor must be > 1*step
	// and within the buffer. Stronger assertion: count is consumed and
	// motion was multiplied (not the unscaled single-page step).
	step := m.logContentHeight()
	assert.Greater(t, rm.logCursor, step, "2<C-f> must move further than a single full page")
	assert.Empty(t, rm.logLineInput)
	assert.False(t, rm.logFollow)
}

// --- Search nav: n / N with count ---
//
// `Nn` jumps to the Nth next match. We seed three matches and verify a count
// of 2 lands on the second-from-cursor match (skipping the first).

func TestDescribeCountPrefixSearchNext(t *testing.T) {
	m := baseModelDescribe()
	m.describeContent = "miss\nhit\nmiss\nhit\nmiss\nhit\nmiss"
	m.describeSearchQuery = "hit"
	m.describeCursor = 0
	m.describeLineInput = "2"
	ret, _ := m.handleDescribeKey(keyMsg("n"))
	rm := ret.(Model)
	// First `n` from row 0 lands on row 1; second `n` lands on row 3.
	assert.Equal(t, 3, rm.describeCursor)
	assert.Empty(t, rm.describeLineInput)
}

func TestLogsCountPrefixSearchNext(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:        []string{"miss", "hit", "miss", "hit", "miss", "hit"},
		logSearchQuery:  "hit",
		logCursor:       0,
		logVisualCurCol: 0,
		logLineInput:    "2",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("n"))
	rm := ret.(Model)
	// 2n from cursor 0: first match at row 1, second at row 3.
	assert.Equal(t, 3, rm.logCursor)
	assert.Empty(t, rm.logLineInput)
}

// --- Event timeline: column and page with count ---

func TestEventTimelineCountPrefixColumnRight(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"alpha beta gamma"},
		eventTimelineCursor:    0,
		eventTimelineCursorCol: 0,
		eventTimelineLineInput: "4",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("l"))
	rm := ret.(Model)
	assert.Equal(t, 4, rm.eventTimelineCursorCol)
	assert.Empty(t, rm.eventTimelineLineInput)
}

func TestEventTimelineCountPrefixHalfPageDown(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "e"
	}
	m := Model{
		width: 80, height: 40, mode: modeEventViewer,
		eventTimelineLines:     lines,
		eventTimelineCursor:    0,
		eventTimelineLineInput: "3",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	step := m.eventContentHeight() / 2
	assert.Equal(t, 3*step, rm.eventTimelineCursor)
	assert.Empty(t, rm.eventTimelineLineInput)
}

// --- Diff: column and page with count ---

func TestDiffCountPrefixColumnRight(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "hello world", diffRight: "hello world",
		diffLeftName: "before", diffRightName: "after",
		diffVisualCurCol: 0,
		diffLineInput:    "5",
		tabs:             []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("l"))
	rm := ret.(Model)
	assert.Equal(t, 5, rm.diffVisualCurCol)
	assert.Empty(t, rm.diffLineInput)
}

func TestDiffCountPrefixHalfPageDown(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "x"
	}
	content := strings.Join(lines, "\n")
	m := Model{
		width: 80, height: 40, mode: modeDiff,
		diffLeft: content, diffRight: content,
		diffLeftName: "before", diffRightName: "after",
		diffCursor:    0,
		diffLineInput: "2",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	// 2 * height/2 = 40 lines.
	assert.Equal(t, 40, rm.diffCursor)
	assert.Empty(t, rm.diffLineInput)
}

// --- YAML: column and page with count ---

func TestYAMLCountPrefixColumnRight(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:      "key: value",
		yamlCollapsed:    map[string]bool{},
		yamlVisualCurCol: yamlFoldPrefixLen,
		yamlLineInput:    "5",
		tabs:             []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("l"))
	rm := ret.(Model)
	assert.Equal(t, yamlFoldPrefixLen+5, rm.yamlVisualCurCol)
	assert.Empty(t, rm.yamlLineInput)
}

func TestYAMLCountPrefixHalfPageDown(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "k: v"
	}
	m := Model{
		width: 80, height: 40, mode: modeYAML,
		yamlContent:   strings.Join(lines, "\n"),
		yamlCollapsed: map[string]bool{},
		yamlCursor:    0,
		yamlLineInput: "2",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	// Step is yamlViewportLines()/2, not m.height/2: the YAML viewer
	// reserves 5 rows of overhead (title/tabs/border/hint), so the
	// viewport at height=40 is 35 and the half-page is 17. 2 * 17 = 34.
	step := m.yamlViewportLines() / 2
	assert.Equal(t, 2*step, rm.yamlCursor)
	assert.Equal(t, 34, rm.yamlCursor, "viewport=35, half=17, 2*17=34")
	assert.Empty(t, rm.yamlLineInput)
}

// --- Diff: search-nav with count ---
//
// `Nn` / `NN` advance the match index by N steps. The matchLines slice is
// seeded directly so we don't have to thread a search through the diff
// engine — the cursor advance is a pure index modulo.

func TestDiffCountPrefixSearchNext(t *testing.T) {
	m := Model{
		width: 80, height: 40, mode: modeDiff,
		diffLeft: "a\nb\nc\nd\ne", diffRight: "a\nb\nc\nd\ne",
		diffLeftName: "before", diffRightName: "after",
		diffMatchLines:  []int{1, 3, 4},
		diffMatchIdx:    0,
		diffSearchQuery: "x",
		diffLineInput:   "2",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("n"))
	rm := ret.(Model)
	// 2n from idx 0: 0 -> 1 -> 2.
	assert.Equal(t, 2, rm.diffMatchIdx)
	assert.Empty(t, rm.diffLineInput)
}

func TestDiffCountPrefixSearchPrev(t *testing.T) {
	m := Model{
		width: 80, height: 40, mode: modeDiff,
		diffLeft: "a\nb\nc\nd\ne", diffRight: "a\nb\nc\nd\ne",
		diffLeftName: "before", diffRightName: "after",
		diffMatchLines:  []int{1, 3, 4},
		diffMatchIdx:    0,
		diffSearchQuery: "x",
		diffLineInput:   "2",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("N"))
	rm := ret.(Model)
	// 2N from idx 0 wraps backward: 0 -> 2 -> 1.
	assert.Equal(t, 1, rm.diffMatchIdx)
	assert.Empty(t, rm.diffLineInput)
}

// --- Event timeline: word motion with count ---

func TestEventTimelineCountPrefixWordForward(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"alpha beta gamma delta"},
		eventTimelineCursor:    0,
		eventTimelineCursorCol: 0,
		eventTimelineLineInput: "2",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("w"))
	rm := ret.(Model)
	// alpha(0) -> beta(6) -> gamma(11): 2w lands at gamma.
	assert.Equal(t, 11, rm.eventTimelineCursorCol)
	assert.Empty(t, rm.eventTimelineLineInput)
}

// --- Buffer-clear on count-ignoring motions ($ / ^) ---
//
// `$` and `^` are absolute-position motions: vim ignores any count prefix.
// The implementation must still consume the buffer so a stray `5` doesn't
// leak forward into the next motion.

func TestDescribeDollarClearsBuffer(t *testing.T) {
	m := baseModelDescribe()
	m.describeContent = "hello world"
	m.describeCursor = 0
	m.describeCursorCol = 0
	m.describeLineInput = "5"
	ret, _ := m.handleDescribeKey(keyMsg("$"))
	rm := ret.(Model)
	assert.Empty(t, rm.describeLineInput, "$ must consume the digit buffer")
	// $ jumps to the last column on the line regardless of the count.
	assert.Equal(t, len([]rune("hello world"))-1, rm.describeCursorCol)
}

func TestDescribeCaretClearsBuffer(t *testing.T) {
	m := baseModelDescribe()
	m.describeContent = "  hello"
	m.describeCursor = 0
	m.describeCursorCol = 6
	m.describeLineInput = "9"
	ret, _ := m.handleDescribeKey(keyMsg("^"))
	rm := ret.(Model)
	assert.Empty(t, rm.describeLineInput, "^ must consume the digit buffer")
	// ^ jumps to the first non-whitespace column (2).
	assert.Equal(t, 2, rm.describeCursorCol)
}

func TestYAMLDollarClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:      "hello world",
		yamlCollapsed:    map[string]bool{},
		yamlCursor:       0,
		yamlVisualCurCol: 0,
		yamlLineInput:    "7",
		tabs:             []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("$"))
	rm := ret.(Model)
	assert.Empty(t, rm.yamlLineInput, "$ must consume the digit buffer")
	assert.Equal(t, len([]rune("hello world"))-1, rm.yamlVisualCurCol)
}

// YAML's `^` is unique among the viewers: it clamps to yamlFoldPrefixLen
// (the fold-marker gutter width) rather than the literal first non-whitespace
// column. Guards both the buffer-clear path and that clamp.
func TestYAMLCaretClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:      "    hello",
		yamlCollapsed:    map[string]bool{},
		yamlCursor:       0,
		yamlVisualCurCol: 8,
		yamlLineInput:    "9",
		tabs:             []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("^"))
	rm := ret.(Model)
	assert.Empty(t, rm.yamlLineInput, "^ must consume the digit buffer")
	// firstNonWhitespace("    hello") = 4, max(4, yamlFoldPrefixLen) = 4.
	assert.Equal(t, 4, rm.yamlVisualCurCol)
}

func TestDiffCaretClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 40, mode: modeDiff,
		diffLeft: "  hello", diffRight: "  hello",
		diffLeftName: "before", diffRightName: "after",
		diffCursor:       0,
		diffVisualCurCol: 6,
		diffLineInput:    "5",
		tabs:             []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("^"))
	rm := ret.(Model)
	assert.Empty(t, rm.diffLineInput, "^ must consume the digit buffer")
	assert.Equal(t, 2, rm.diffVisualCurCol)
}

func TestLogsDollarClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:        []string{"hello world"},
		logCursor:       0,
		logVisualCurCol: 0,
		logLineInput:    "5",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("$"))
	rm := ret.(Model)
	assert.Empty(t, rm.logLineInput, "$ must consume the digit buffer")
	assert.Equal(t, len([]rune("hello world"))-1, rm.logVisualCurCol)
}

func TestLogsCaretClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:        []string{"  hello"},
		logCursor:       0,
		logVisualCurCol: 6,
		logLineInput:    "9",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("^"))
	rm := ret.(Model)
	assert.Empty(t, rm.logLineInput, "^ must consume the digit buffer")
	assert.Equal(t, 2, rm.logVisualCurCol)
}

func TestEventTimelineDollarClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"hello world"},
		eventTimelineCursor:    0,
		eventTimelineCursorCol: 0,
		eventTimelineLineInput: "5",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("$"))
	rm := ret.(Model)
	assert.Empty(t, rm.eventTimelineLineInput, "$ must consume the digit buffer")
	assert.Equal(t, len([]rune("hello world"))-1, rm.eventTimelineCursorCol)
}

func TestEventTimelineCaretClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"  hello"},
		eventTimelineCursor:    0,
		eventTimelineCursorCol: 6,
		eventTimelineLineInput: "9",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKey(keyMsg("^"))
	rm := ret.(Model)
	assert.Empty(t, rm.eventTimelineLineInput, "^ must consume the digit buffer")
	assert.Equal(t, 2, rm.eventTimelineCursorCol)
}

// Companion to TestLogsCountPrefixColumnRight: `Nh` must clamp at column 0
// rather than walking negative.
func TestLogsCountPrefixColumnLeftClampsAtZero(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:        []string{"hello world from logs"},
		logCursor:       0,
		logVisualCurCol: 3,
		logLineInput:    "100",
		tabs:            []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("h"))
	rm := ret.(Model)
	assert.Equal(t, 0, rm.logVisualCurCol)
	assert.Empty(t, rm.logLineInput)
}

// Pins the diffEnterVisual fix: a digit buffer typed before `v` must not leak
// past the visual-mode toggle. Visual-mode page/word handlers don't consume
// counts, so a stale prefix would otherwise multiply the next normal-mode
// motion after `<Esc>`.
func TestDiffEnterVisualClearsBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 40, mode: modeDiff,
		diffLeft: "a\nb\nc\nd\ne", diffRight: "a\nb\nc\nd\ne",
		diffLeftName: "before", diffRightName: "after",
		diffCursor:    0,
		diffLineInput: "5",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("v"))
	rm := ret.(Model)
	assert.True(t, rm.diffVisualMode)
	assert.Empty(t, rm.diffLineInput, "entering visual mode must consume the digit buffer")
}
