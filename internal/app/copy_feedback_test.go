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
		yamlView: yamlViewState{
			content:   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test",
			collapsed: map[string]bool{},
		},
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
		diffView: diffViewState{
			left: "a: 1\nb: 2", right: "a: 1\nb: 3",
			leftName: "before", rightName: "after",
		},
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
		yamlView: yamlViewState{
			content:   "apiVersion: v1\nkind: Pod\nmetadata:\n  name: test",
			collapsed: map[string]bool{},
			cursor:    1,
		},
		tabs: []TabState{{}},
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
		diffView: diffViewState{
			left: "a: 1\nb: 2\nc: 3", right: "a: 1\nb: 2\nc: 4",
			leftName: "before", rightName: "after",
			cursor: 2,
		},
		tabs: []TabState{{}},
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
		logView: logViewState{
			lines:  []string{"line one", "line two", "line three"},
			cursor: 1,
		},
		tabs: []TabState{{}},
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
		logView: logViewState{
			lines: nil, cursor: 0,
		},
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
		yamlView: yamlViewState{
			content:   "a: 1\nb: 2\nc: 3\nd: 4\ne: 5",
			collapsed: map[string]bool{},
			cursor:    1,
			lineInput: "3",
		},
		tabs: []TabState{{}},
	}
	ret, cmd := m.handleYAMLKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.yamlView.lineInput, "digit buffer must be consumed by the yank")
	assert.NotNil(t, cmd)
}

func TestDescribeNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := baseModelDescribe()
	m.describeView.cursor = 2
	m.describeView.lineInput = "4"
	ret, cmd := m.handleDescribeKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 4 lines", rm.statusMessage)
	assert.Empty(t, rm.describeView.lineInput)
	assert.NotNil(t, cmd)
}

func TestLogsNormalCopyCountPrefixYanksMultipleLines(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logView: logViewState{
			lines:     []string{"a", "b", "c", "d", "e", "f"},
			cursor:    1,
			lineInput: "3",
		},
		tabs: []TabState{{}},
	}
	ret, cmd := m.handleLogKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.logView.lineInput)
	assert.NotNil(t, cmd)
}

// `100y` near end-of-file must clamp to the lines that actually exist
// rather than reporting the requested count.
func TestLogsNormalCopyCountClampsToRemaining(t *testing.T) {
	m := Model{
		width: 80, height: 30, mode: modeLogs,
		logView: logViewState{
			lines:     []string{"a", "b", "c"},
			cursor:    1,
			lineInput: "100",
		},
		tabs: []TabState{{}},
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
		diffView: diffViewState{
			left: "a: 1\nb: 2\nc: 3\nd: 4\ne: 5", right: "a: 1\nb: 2\nc: 3\nd: 4\ne: 5",
			leftName: "before", rightName: "after",
			cursor:    1,
			lineInput: "3",
		},
		tabs: []TabState{{}},
	}
	ret, cmd := m.handleDiffKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage)
	assert.Empty(t, rm.diffView.lineInput)
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
		yamlView: yamlViewState{
			content:   content,
			sections:  sections,
			collapsed: collapsed,
			cursor:    0,
			lineInput: "100",
		},
		tabs: []TabState{{}},
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
		diffView: diffViewState{
			left: "a\nb\nc", right: "a\nx\ny\nb\nc",
			leftName: "before", rightName: "after",
			cursorSide: 0,
			cursor:     0,
			lineInput:  "100",
		},
		tabs: []TabState{{}},
	}
	ret, _ := m.handleDiffKey(keyMsg("y"))
	rm := ret.(Model)
	assert.Equal(t, "Copied 3 lines", rm.statusMessage,
		"left side has 3 real lines; the 2 insert rows must be skipped")
}

// Regression guard for issue #261: on Windows the clipboard convention
// is CRLF (CF_UNICODETEXT), so payloads built with bare LF — notably the
// "copy as table" output — paste as a single long line in Notepad,
// Excel, and many browser textareas. Normalize to CRLF on Windows;
// leave other platforms untouched.
func TestNormalizeClipboardLineEndings(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		text string
		goos string
		want string
	}{
		{"non-windows untouched", "a\nb\nc", "linux", "a\nb\nc"},
		{"darwin untouched", "a\nb\nc", "darwin", "a\nb\nc"},
		{"windows lf to crlf", "a\nb\nc", "windows", "a\r\nb\r\nc"},
		{"windows preserves existing crlf", "a\r\nb\r\nc", "windows", "a\r\nb\r\nc"},
		{"windows mixed normalized", "a\r\nb\nc", "windows", "a\r\nb\r\nc"},
		{"windows no newlines noop", "abc", "windows", "abc"},
		{"empty", "", "windows", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeClipboardLineEndings(tt.text, tt.goos)
			assert.Equal(t, tt.want, got)
		})
	}
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
	// On hosts where atotto/clipboard can't reach a clipboard (Linux CI
	// without xsel/xclip/wl-copy installed, headless containers, etc.) an
	// error is expected — only assert success-path silence when the write
	// actually succeeded.
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
	m.describeView.searchQuery = "Name"
	m.statusMessage = "Copied 1 line"
	m.statusMessageExp = time.Now().Add(5 * time.Second)
	out := stripANSI(m.View())
	assert.Contains(t, out, "Copied 1 line")
	// Search overlay shouldn't claim the footer simultaneously.
	lines := strings.Split(out, "\n")
	footer := lines[len(lines)-1]
	assert.NotContains(t, footer, "/Name")
}
