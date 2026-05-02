package app

import (
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
