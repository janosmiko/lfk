package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
)

// --- logMaxScroll with wrap and line numbers ---

func TestLogMaxScrollWrapWithLineNumbers(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "a short line"
	}
	m := Model{
		height: 20,
		width:  80,
		tabs:   []TabState{{}},
		logView: logViewState{
			lines:       lines,
			wrap:        true,
			lineNumbers: true,
		},
	}
	ms := m.logMaxScroll()
	assert.GreaterOrEqual(t, ms, 0)
}

// --- viewDescribe: scroll past end ---

func TestViewDescribeScrollPastEnd(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		describeView: describeViewState{
			title:   "Test",
			content: "line1\nline2\nline3",
			scroll:  100,
		},
	}
	output := m.viewDescribe()
	assert.NotEmpty(t, output)
}

func TestViewDescribeNegativeScroll(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
		describeView: describeViewState{
			title:   "Test",
			content: "line1\nline2\nline3",
			scroll:  -5,
		},
	}
	output := m.viewDescribe()
	assert.NotEmpty(t, output)
}

// --- viewExplain ---

func TestViewExplainSearchActive(t *testing.T) {
	m := Model{
		width:               120,
		height:              30,
		mode:                modeExplain,
		explainTitle:        "Explain: Pod",
		explainDesc:         "A Pod is the smallest deployable unit.",
		explainSearchActive: true,
		explainSearchInput:  TextInput{Value: "spec"},
		explainFields: []model.ExplainField{
			{Name: "spec", Type: "<PodSpec>", Description: "Pod specification"},
		},
		tabs: []TabState{{}},
	}
	output := m.viewExplain()
	assert.NotEmpty(t, output)
}

func TestViewExplainSearchQuery(t *testing.T) {
	m := Model{
		width:              120,
		height:             30,
		mode:               modeExplain,
		explainTitle:       "Explain: Pod",
		explainDesc:        "A Pod is the smallest deployable unit.",
		explainSearchQuery: "containers",
		explainFields: []model.ExplainField{
			{Name: "spec", Type: "<PodSpec>", Description: "Pod specification"},
		},
		tabs: []TabState{{}},
	}
	output := m.viewExplain()
	assert.NotEmpty(t, output)
}

// --- viewExecTerminal: nil terminal ---

func TestViewExecTerminalNilTerm(t *testing.T) {
	m := Model{
		width:     80,
		height:    30,
		mode:      modeExec,
		execTitle: "Exec: my-pod",
		tabs:      []TabState{{}},
	}
	output := m.viewExecTerminal()
	stripped := stripANSI(output)
	assert.Contains(t, stripped, "Terminal not initialized")
}

// --- ensureLogCursorVisible: edge cases ---

func TestEnsureLogCursorVisibleEmptyLog(t *testing.T) {
	m := Model{
		height: 20,
		width:  80,
		tabs:   []TabState{{}},
		logView: logViewState{
			cursor: 5,
		},
	}
	m.ensureLogCursorVisible()
	// With no log lines, cursor is clamped to -1.
	// No panic expected.
	assert.NotNil(t, m)
}

// --- clampLogScroll with line numbers enabled ---

func TestClampLogScrollWithLineNumbers(t *testing.T) {
	lines := make([]string, 50)
	for i := range lines {
		lines[i] = "log line content"
	}
	m := Model{
		height: 20,
		width:  80,
		tabs:   []TabState{{}},
		logView: logViewState{
			lines:       lines,
			wrap:        true,
			lineNumbers: true,
			scroll:      200,
		},
	}
	m.clampLogScroll()
	assert.GreaterOrEqual(t, m.logView.scroll, 0)
	assert.LessOrEqual(t, m.logView.scroll, len(lines))
}
