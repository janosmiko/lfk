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
// `N<C-d>` follows vim's sticky-scroll semantics: it moves exactly N lines
// (clamped to viewport) and remembers N as the 'scroll' value for future
// uncounted <C-d>/<C-u> presses. `N<C-f>` scales full-page motion by N.
// The dedicated stickiness / clamp / default tests below
// (TestLogsCtrlD*, TestDescribeVimScrollSemantics) pin the rest of the
// behavior; the per-viewer tests in this section only assert the basic
// "counted press moves count lines and sets sticky=count" contract.

func TestDescribeCountPrefixHalfPageDown(t *testing.T) {
	m := baseModelDescribe()
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line"
	}
	m.describeContent = strings.Join(lines, "\n")
	m.describeCursor = 0
	m.describeLineInput = "2"
	ret, _ := m.handleDescribeKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	// Vim semantics: `[count]<C-d>` moves exactly count lines (clamped to
	// viewport) and stores count as the sticky 'scroll' option for future
	// uncounted <C-d>/<C-u> presses.
	assert.Equal(t, 2, rm.describeCursor)
	assert.Equal(t, 2, rm.describeScrollOption)
	assert.Empty(t, rm.describeLineInput)
}

// Vim's `[count]<C-d>` does NOT equal repeated single presses. A counted
// press sets the sticky 'scroll' to count and moves count lines. Plain
// presses then reuse the sticky value. So `2<C-d>` moves 2; `<C-d><C-d>`
// (no count, default = half-viewport) moves twice the half-viewport. They
// only coincide if count happens to equal the default. Pin both paths so a
// future regression toward "count multiplies the step" gets caught.
func TestDescribeVimScrollSemantics(t *testing.T) {
	m := baseModelDescribe()
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line"
	}
	m.describeContent = strings.Join(lines, "\n")
	m.describeCursor = 0

	// Counted: 2<C-d> moves exactly 2 lines, sets sticky=2.
	m.describeLineInput = "2"
	ret, _ := m.handleDescribeKey(keyMsg("ctrl+d"))
	counted := ret.(Model)
	assert.Equal(t, 2, counted.describeCursor)
	assert.Equal(t, 2, counted.describeScrollOption)

	// Subsequent plain <C-d> reuses sticky scroll=2 (vim's stickiness).
	ret, _ = counted.handleDescribeKey(keyMsg("ctrl+d"))
	stuck := ret.(Model)
	assert.Equal(t, 4, stuck.describeCursor, "plain <C-d> must reuse sticky scroll value")

	// Reference: two un-counted presses use the default (half-viewport).
	ref := m
	ref.describeLineInput = ""
	ref.describeScrollOption = 0
	ret, _ = ref.handleDescribeKey(keyMsg("ctrl+d"))
	ref = ret.(Model)
	ret, _ = ref.handleDescribeKey(keyMsg("ctrl+d"))
	ref = ret.(Model)
	half := ref.describeContentHeight() / 2
	assert.Equal(t, 2*half, ref.describeCursor, "two plain <C-d> = 2 * half-viewport")
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
	// Vim semantics: 3<C-d> moves 3 lines and sets sticky scroll=3.
	assert.Equal(t, 3, rm.eventTimelineCursor)
	assert.Equal(t, 3, rm.eventTimelineScrollOption)
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
	// Vim semantics: 2<C-d> moves 2 lines and sets sticky scroll=2.
	assert.Equal(t, 2, rm.diffCursor)
	assert.Equal(t, 2, rm.diffScrollOption)
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
	// Vim semantics: 2<C-d> moves 2 lines and sets sticky scroll=2.
	assert.Equal(t, 2, rm.yamlCursor)
	assert.Equal(t, 2, rm.yamlScrollOption)
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

// --- Vim 'scroll' semantics for [count]<C-d>/<C-u> ---
//
// Vim's [count]<C-d> first sets 'scroll' to min(count, winheight), then
// scrolls by that amount. The new value is sticky: subsequent uncounted
// <C-d>/<C-u> presses reuse it, until another counted press (on either key)
// replaces it. The same option is shared between <C-d> and <C-u>.
//
// Empirically verified against vim 9.1 and nvim 0.12: in a 23-line window
// `5<C-d>` moves 5 lines and sets &scroll=5; subsequent `<C-d>` moves 5 more;
// `<C-u>` moves 5 back; `999<C-d>` clamps to winheight.

func longLogModel() Model {
	lines := make([]string, 1000)
	for i := range lines {
		lines[i] = "x"
	}
	return Model{
		width: 80, height: 40, mode: modeLogs,
		logLines: lines,
		tabs:     []TabState{{}},
	}
}

// Plain <C-d> with no prior counted press uses the default 'scroll' value
// (half the viewport), matching vim's `default: half a screen`.
func TestLogsCtrlDDefaultsToHalfViewport(t *testing.T) {
	m := longLogModel()
	m.logCursor = 0
	ret, _ := m.handleLogKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	assert.Equal(t, m.logContentHeight()/2, rm.logCursor)
	assert.Equal(t, 0, rm.logScrollOption, "plain <C-d> must not change the sticky option")
}

// 5<C-d> sets sticky scroll=5 and moves 5 lines. The next plain <C-d> reuses
// 5. The next plain <C-u> moves 5 back up. Then 8<C-u> replaces sticky=8 and
// moves 8. The next plain <C-d> uses the new 8.
func TestLogsCtrlDStickyAndShared(t *testing.T) {
	m := longLogModel()
	m.logCursor = 0

	m.logLineInput = "5"
	ret, _ := m.handleLogKey(keyMsg("ctrl+d"))
	a := ret.(Model)
	assert.Equal(t, 5, a.logCursor)
	assert.Equal(t, 5, a.logScrollOption)

	ret, _ = a.handleLogKey(keyMsg("ctrl+d"))
	b := ret.(Model)
	assert.Equal(t, 10, b.logCursor, "plain <C-d> reuses sticky=5")
	assert.Equal(t, 5, b.logScrollOption)

	ret, _ = b.handleLogKey(keyMsg("ctrl+u"))
	c := ret.(Model)
	assert.Equal(t, 5, c.logCursor, "plain <C-u> shares the same sticky")

	c.logLineInput = "8"
	ret, _ = c.handleLogKey(keyMsg("ctrl+u"))
	d := ret.(Model)
	assert.Equal(t, 0, d.logCursor, "8<C-u> from line 5 clamps at top")
	assert.Equal(t, 8, d.logScrollOption, "counted <C-u> replaces sticky")

	ret, _ = d.handleLogKey(keyMsg("ctrl+d"))
	e := ret.(Model)
	assert.Equal(t, 8, e.logCursor, "plain <C-d> uses new sticky=8")
}

// Vim caps 'scroll' at winheight; mirror that with the lfk viewport.
func TestLogsCtrlDClampsToViewport(t *testing.T) {
	m := longLogModel()
	m.logCursor = 0
	m.logLineInput = "999"
	ret, _ := m.handleLogKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	viewport := m.logContentHeight()
	assert.Equal(t, viewport, rm.logCursor, "999<C-d> caps at viewport")
	assert.Equal(t, viewport, rm.logScrollOption, "sticky cap matches viewport")
}

// The scroll option is per-viewer: setting log's sticky must not change the
// describe / yaml / diff / events sticky values.
func TestScrollOptionIsPerViewer(t *testing.T) {
	m := longLogModel()
	m.logLineInput = "7"
	ret, _ := m.handleLogKey(keyMsg("ctrl+d"))
	rm := ret.(Model)
	assert.Equal(t, 7, rm.logScrollOption)
	assert.Equal(t, 0, rm.describeScrollOption)
	assert.Equal(t, 0, rm.yamlScrollOption)
	assert.Equal(t, 0, rm.diffScrollOption)
	assert.Equal(t, 0, rm.eventTimelineScrollOption)
}
