package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSplitLogPreviewWidth(t *testing.T) {
	cases := []struct {
		name              string
		total             int
		wantLog, wantPrev int
	}{
		{"narrow disables panel", 60, 60, 0},
		{"borderline disables panel", 79, 79, 0},
		{"baseline 80 splits to floor", 80, 50, 30},
		{"medium 120 hits 40 percent", 120, 72, 48},
		{"wide caps preview at 80", 250, 170, 80},
		{"width 100 keeps natural split", 100, 60, 40},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotLog, gotPrev := splitLogPreviewWidth(tc.total)
			assert.Equal(t, tc.wantLog, gotLog, "logWidth")
			assert.Equal(t, tc.wantPrev, gotPrev, "previewWidth")
			if gotPrev > 0 {
				assert.Equal(t, tc.total, gotLog+gotPrev, "split must sum to total")
			}
		})
	}
}

func TestLogEffectiveWidthRespectsPreview(t *testing.T) {
	m := Model{width: 200}
	assert.Equal(t, 200, m.logEffectiveWidth(), "preview off: full width")

	m.logPreviewVisible = true
	logW, prevW := splitLogPreviewWidth(200)
	assert.Equal(t, logW, m.logEffectiveWidth(), "preview on: log column width")
	assert.Greater(t, prevW, 0, "expected non-zero preview width at width 200")

	m.width = 60
	assert.Equal(t, 60, m.logEffectiveWidth(), "narrow terminal: preview hidden, full width")
}

func TestLogPreviewLine(t *testing.T) {
	m := Model{logLines: nil}
	assert.Equal(t, "", m.logPreviewLine(), "empty buffer returns empty")

	m.logLines = []string{"first", "second", "third"}
	m.logCursor = 1
	assert.Equal(t, "second", m.logPreviewLine(), "cursor in range returns its line")

	m.logCursor = -1
	assert.Equal(t, "third", m.logPreviewLine(), "unset cursor falls back to last line")

	m.logCursor = 99
	assert.Equal(t, "third", m.logPreviewLine(), "out-of-range cursor falls back to last line")
}

func TestLogKeyPCapitalTogglesPreview(t *testing.T) {
	m := Model{
		mode:     modeLogs,
		logLines: []string{`{"level":"info","msg":"hi"}`},
		tabs:     []TabState{{}},
		width:    120,
		height:   40,
	}
	assert.False(t, m.logPreviewVisible)

	ret, _ := m.handleLogKey(runeKey('P'))
	on := ret.(Model)
	assert.True(t, on.logPreviewVisible, "P should turn preview on")

	ret2, _ := on.handleLogKey(runeKey('P'))
	off := ret2.(Model)
	assert.False(t, off.logPreviewVisible, "second P should turn preview off")
}

func TestLogKeyPLowercaseStillTogglesPrefixesOnly(t *testing.T) {
	// Regression guard: lowercase p must remain bound to the prefix toggle
	// and must not affect the new preview state.
	m := Model{
		mode:     modeLogs,
		logLines: []string{"[pod/x/y] line"},
		tabs:     []TabState{{}},
		width:    120,
		height:   40,
	}
	ret, _ := m.handleLogKey(runeKey('p'))
	result := ret.(Model)
	assert.True(t, result.logHidePrefixes, "lowercase p toggles prefixes")
	assert.False(t, result.logPreviewVisible, "lowercase p must not affect preview")
}

func TestViewLogsRendersPreviewWhenVisible(t *testing.T) {
	m := Model{
		mode:              modeLogs,
		logLines:          []string{`{"level":"info","msg":"served","port":8080}`},
		logCursor:         0,
		logPreviewVisible: true,
		tabs:              []TabState{{}},
		width:             140,
		height:            30,
		logTitle:          "Logs",
	}
	out := m.viewLogs()
	// The PREVIEW header and the JSON-detection label should appear once
	// the side panel is rendered.
	assert.Contains(t, out, "PREVIEW")
	assert.Contains(t, out, "JSON")
}

func TestViewLogsHidesPreviewWhenTerminalTooNarrow(t *testing.T) {
	m := Model{
		mode:              modeLogs,
		logLines:          []string{`{"level":"info"}`},
		logCursor:         0,
		logPreviewVisible: true, // armed but width < 80 so panel must be suppressed
		tabs:              []TabState{{}},
		width:             60,
		height:            30,
		logTitle:          "Logs",
	}
	out := m.viewLogs()
	assert.NotContains(t, out, "PREVIEW", "panel must be suppressed when terminal is too narrow")
}
