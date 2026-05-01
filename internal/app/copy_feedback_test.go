package app

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Each fullscreen viewer's footer must surface a fresh status message in
// place of its hint bar — copy feedback set by `y` is the motivating case.
// Without these the user copies a line and gets no on-screen confirmation.

func TestViewDescribeShowsStatusMessage(t *testing.T) {
	m := baseModelDescribe()
	m.statusMessage = "Copied 1 line"
	m.statusMessageExp = time.Now().Add(5 * time.Second)
	out := stripANSI(m.View())
	assert.Contains(t, out, "Copied 1 line")
}

func TestViewYAMLShowsStatusMessage(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:      "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test",
		yamlCollapsed:    map[string]bool{},
		tabs:             []TabState{{}},
		statusMessage:    "Copied 1 line",
		statusMessageExp: time.Now().Add(5 * time.Second),
	}
	out := stripANSI(m.View())
	assert.Contains(t, out, "Copied 1 line")
}

func TestViewDiffShowsStatusMessage(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a: 1\nb: 2", diffRight: "a: 1\nb: 3",
		diffLeftName: "before", diffRightName: "after",
		tabs:             []TabState{{}},
		statusMessage:    "Copied 1 line",
		statusMessageExp: time.Now().Add(5 * time.Second),
	}
	out := stripANSI(m.View())
	assert.Contains(t, out, "Copied 1 line")
}

func TestViewEventViewerShowsStatusMessage(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines: []string{"event 1", "event 2"},
		tabs:               []TabState{{}},
		statusMessage:      "Copied 1 line",
		statusMessageExp:   time.Now().Add(5 * time.Second),
	}
	out := stripANSI(m.View())
	assert.Contains(t, out, "Copied 1 line")
}

// Normal-mode `y` previously had no binding in YAML/diff/logs. Each handler
// must yank the cursor's line and surface a status message — the same
// vim-style behaviour the describe view already had.

func TestYAMLNormalCopyYanksCursorLine(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test",
		yamlCollapsed: map[string]bool{},
		yamlCursor:    1,
		tabs:          []TabState{{}},
	}
	ret, cmd := m.handleYAMLKey(keyMsg("y"))
	rm := ret.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.Contains(t, rm.statusMessage, "Copied 1 line")
	assert.NotNil(t, cmd) // tea.Batch(copy, scheduleStatusClear)
}

func TestDiffNormalCopyYanksCursorLine(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a: 1\nb: 2\nc: 3", diffRight: "a: 1\nb: 2\nc: 4",
		diffLeftName: "before", diffRightName: "after",
		diffCursor: 2,
		tabs:       []TabState{{}},
	}
	ret, cmd := m.handleDiffKey(keyMsg("y"))
	rm := ret.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.Contains(t, rm.statusMessage, "Copied 1 line")
	assert.NotNil(t, cmd)
}

func TestLogsNormalCopyYanksCursorLine(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:  []string{"line one", "line two", "line three"},
		logCursor: 1,
		tabs:      []TabState{{}},
	}
	ret, cmd := m.handleLogKey(keyMsg("y"))
	rm := ret.(Model)
	assert.True(t, rm.hasStatusMessage())
	assert.Contains(t, rm.statusMessage, "Copied 1 line")
	assert.NotNil(t, cmd)
}

// Sanity check: an empty buffer should not crash or claim a copy happened.
func TestLogsNormalCopyEmptyBuffer(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines: nil, logCursor: 0,
		tabs: []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("y"))
	rm := ret.(Model)
	assert.False(t, rm.hasStatusMessage())
}

// A digit-prefix yank (e.g. `123y`) reuses the same digit accumulator that
// powers `123G` jump-to-line. The buffer must be consumed by the yank, the
// status must reflect the actual line count, and the count must clamp to
// the remaining content rather than walking off the end.

func TestYAMLNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   "a: 1\nb: 2\nc: 3\nd: 4\ne: 5",
		yamlCollapsed: map[string]bool{},
		yamlCursor:    1,
		yamlLineInput: "3",
		tabs:          []TabState{{}},
	}
	ret, cmd := m.handleYAMLKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.yamlLineInput, "digit buffer must be consumed by the yank")
	assert.NotNil(t, cmd)
}

func TestDescribeNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := baseModelDescribe()
	m.describeCursor = 2
	m.describeLineInput = "4"
	ret, cmd := m.handleDescribeKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 4 lines", rm.statusMessage)
	assert.Empty(t, rm.describeLineInput)
	assert.NotNil(t, cmd)
}

func TestLogsNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     []string{"a", "b", "c", "d", "e", "f"},
		logCursor:    1,
		logLineInput: "3",
		tabs:         []TabState{{}},
	}
	ret, cmd := m.handleLogKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.logLineInput)
	assert.NotNil(t, cmd)
}

// `100y` near end-of-file must clamp to the lines that actually exist
// rather than reporting the requested count.
func TestLogsNormalCopyCountClampsToRemaining(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logLines:     []string{"a", "b", "c"},
		logCursor:    1,
		logLineInput: "100",
		tabs:         []TabState{{}},
	}
	ret, _ := m.handleLogKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 2 lines", rm.statusMessage)
}

// Diff and event-timeline viewers use the same shape (digit accumulator +
// single-line `y` handler), so count-prefixed yank must light up there too.

func TestDiffNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a: 1\nb: 2\nc: 3\nd: 4\ne: 5", diffRight: "a: 1\nb: 2\nc: 3\nd: 4\ne: 5",
		diffLeftName: "before", diffRightName: "after",
		diffCursor:    1,
		diffLineInput: "3",
		tabs:          []TabState{{}},
	}
	ret, cmd := m.handleDiffKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.diffLineInput)
	assert.NotNil(t, cmd)
}

func TestEventTimelineNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"e0", "e1", "e2", "e3", "e4"},
		eventTimelineCursor:    1,
		eventTimelineLineInput: "3",
		tabs:                   []TabState{{}},
	}
	ret, cmd := m.handleEventTimelineOverlayKeyY()
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.eventTimelineLineInput)
	assert.NotNil(t, cmd)
}

func TestEventTimelineNormalCopyClampsAtEnd(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeEventViewer,
		eventTimelineLines:     []string{"e0", "e1", "e2"},
		eventTimelineCursor:    1,
		eventTimelineLineInput: "100",
		tabs:                   []TabState{{}},
	}
	ret, _ := m.handleEventTimelineOverlayKeyY()
	rm := ret.(Model)
	assert.Equal(t, "Copied 2 lines", rm.statusMessage)
}

// When a YAML section is collapsed, its child lines drop out of the
// visible mapping entirely — `Ny` clamps to the visible reach, not the
// raw line count, so the status reports fewer lines than requested.
// Regression guard for the doc-comment claim that a count "straddling
// a fold still copies real content".
func TestYAMLNormalCopyCountSkipsCollapsedSection(t *testing.T) {
	content := "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\n  labels:\n    app: nginx\nspec:\n  replicas: 3"
	sections := parseYAMLSections(content)
	collapsed := map[string]bool{}
	for _, sec := range sections {
		if sec.key == "metadata" {
			collapsed[sec.key] = true
		}
	}
	m := Model{
		width: 80, height: 30, mode: modeYAML,
		yamlContent:   content,
		yamlSections:  sections,
		yamlCollapsed: collapsed,
		yamlCursor:    0,
		yamlLineInput: "100",
		tabs:          []TabState{{}},
	}
	ret, _ := m.handleYAMLKey(keyMsg("y"))
	rm := ret.(Model)
	_, mapping := buildVisibleLines(content, sections, collapsed)
	assert.Less(t, len(mapping), strings.Count(content, "\n")+1,
		"fixture must actually fold something for this test to be meaningful")
	assert.Equal(t, formatCopiedLines(len(mapping)), rm.statusMessage)
}

// Side-by-side diff with insertions on the right side leaves the active
// (left) side empty for the inserted rows. `Ny` skips those empty rows
// so the status reports only the lines that have real content on the
// active side.
func TestDiffNormalCopySkipsEmptySideLines(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeDiff,
		diffLeft: "a\nb\nc", diffRight: "a\nx\ny\nb\nc",
		diffLeftName: "before", diffRightName: "after",
		diffCursorSide: 0,
		diffCursor:     0,
		diffLineInput:  "100",
		tabs:           []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage,
		"left side has 3 real lines; the 2 insert rows must be skipped")
}

// Regression guard: copyToSystemClipboard must not return a generic
// "Copied to clipboard" message — every caller has already set a
// context-specific status. Returning the generic one races back via
// updateActionResult and overwrites the more useful caller message
// (visible to the user as "Copied 1 line" → "Copied to clipboard").
func TestCopyToSystemClipboardSuccessIsSilent(t *testing.T) {
	cmd := copyToSystemClipboard("anything")
	if cmd == nil {
		t.Fatal("copyToSystemClipboard returned nil cmd")
	}
	msg := cmd()
	// On platforms without pbcopy/xclip, an error is expected — only
	// assert success-path silence when the subprocess actually ran.
	if msg == nil {
		return
	}
	res, ok := msg.(actionResultMsg)
	if !ok {
		t.Fatalf("unexpected message type: %T", msg)
	}
	assert.NotEmpty(t, res.err, "non-nil success message would race and overwrite caller status")
}

// Regression guard: the status message must not be muted when a search
// query is also committed in the YAML/describe viewers — the copy
// feedback should win over the search bar.
func TestStatusBeatsSearchBarInDescribe(t *testing.T) {
	m := baseModelDescribe()
	m.describeSearchQuery = "Name"
	m.statusMessage = "Copied 1 line"
	m.statusMessageExp = time.Now().Add(5 * time.Second)
	out := stripANSI(m.View())
	assert.Contains(t, out, "Copied 1 line")
	// Search overlay shouldn't claim the footer simultaneously.
	lines := strings.Split(out, "\n")
	footer := lines[len(lines)-1]
	assert.NotContains(t, footer, "/Name")
}
