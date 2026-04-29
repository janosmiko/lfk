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
