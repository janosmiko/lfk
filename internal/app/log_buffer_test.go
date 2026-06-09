package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
	"github.com/stretchr/testify/assert"
)

// withLogMaxLines sets ConfigLogMaxLines for the duration of a test and
// restores it afterwards.
func withLogMaxLines(t *testing.T, n int) {
	t.Helper()
	orig := ui.ConfigLogMaxLines
	ui.ConfigLogMaxLines = n
	t.Cleanup(func() { ui.ConfigLogMaxLines = orig })
}

func TestCapLogLines_NoTrimUnderCapPlusSlack(t *testing.T) {
	withLogMaxLines(t, 10)
	buf := make([]string, 10+logBufferTrimSlack) // exactly at the threshold
	out, drop := capLogLines(buf)
	assert.Equal(t, 0, drop, "no trim at cap+slack")
	assert.Len(t, out, 10+logBufferTrimSlack)
}

func TestCapLogLines_TrimsToCap(t *testing.T) {
	withLogMaxLines(t, 10)
	total := 10 + logBufferTrimSlack + 5
	buf := make([]string, total)
	for i := range buf {
		buf[i] = string(rune('a' + i%26))
	}
	want := append([]string(nil), buf[total-10:]...)

	out, drop := capLogLines(buf)
	assert.Equal(t, total-10, drop, "drops everything past the cap")
	assert.Len(t, out, 10, "retains exactly the cap")
	assert.Equal(t, want, out, "keeps the newest lines")
}

func TestCapLogLines_DisabledWhenCapNonPositive(t *testing.T) {
	withLogMaxLines(t, 0)
	buf := make([]string, 100000)
	out, drop := capLogLines(buf)
	assert.Equal(t, 0, drop)
	assert.Len(t, out, 100000)
}

func TestShiftLogOffset(t *testing.T) {
	tests := []struct {
		offset, drop, want int
	}{
		{-1, 5, -1}, // inactive cursor preserved
		{0, 5, 0},   // already at top
		{3, 5, 0},   // inside the dropped region clamps to top
		{5, 5, 0},   // boundary clamps to top
		{10, 4, 6},  // shifted back by drop
		{100, 50, 50},
	}
	for _, tc := range tests {
		assert.Equalf(t, tc.want, shiftLogOffset(tc.offset, tc.drop),
			"shiftLogOffset(%d, %d)", tc.offset, tc.drop)
	}
}

// TestUpdateLogLine_ActivePathBounded is the regression guard for issue #387:
// streaming many lines into the active log view must not grow without bound.
func TestUpdateLogLine_ActivePathBounded(t *testing.T) {
	withLogMaxLines(t, 10)
	ch := make(chan string)
	m := Model{
		mode:    modeLogs,
		width:   80,
		height:  40,
		tabs:    []TabState{{}},
		logView: logViewState{ch: ch, cursor: 2, scroll: 1, follow: false, wrap: true, wrapTopSkip: 3},
	}

	total := 10 + logBufferTrimSlack + 200
	for range total {
		ret, _ := m.updateLogLine(logLineMsg{line: "x", ch: ch})
		m = ret.(Model)
	}

	assert.LessOrEqual(t, len(m.logView.lines), 10+logBufferTrimSlack,
		"live buffer must stay bounded, not grow to %d", total)
	// cursor (2) and scroll (1) were inside the first dropped region -> clamped.
	assert.Equal(t, 0, m.logView.cursor, "cursor clamped after trim")
	assert.Equal(t, 0, m.logView.scroll, "scroll clamped after trim")
	assert.Equal(t, 0, m.logView.wrapTopSkip, "wrapTopSkip reset after trim")
}

// TestUpdateLogLine_BackgroundTabBounded guards the background-tab drain path
// (a tab whose log stream is still live while another tab is focused).
func TestUpdateLogLine_BackgroundTabBounded(t *testing.T) {
	withLogMaxLines(t, 10)
	bgCh := make(chan string)
	activeCh := make(chan string)
	m := Model{
		mode:      modeLogs,
		width:     80,
		height:    40,
		activeTab: 0,
		tabs: []TabState{
			{}, // active tab
			{logCh: bgCh, logCursor: 3, logScroll: 2, logWrapTopSkip: 4}, // background tab streaming
		},
		logView: logViewState{ch: activeCh},
	}

	total := 10 + logBufferTrimSlack + 50
	for range total {
		ret, _ := m.updateLogLine(logLineMsg{line: "y", ch: bgCh})
		m = ret.(Model)
	}

	assert.LessOrEqual(t, len(m.tabs[1].logLines), 10+logBufferTrimSlack,
		"background buffer must stay bounded")
	assert.Equal(t, 0, m.tabs[1].logCursor, "background cursor clamped after trim")
	assert.Equal(t, 0, m.tabs[1].logScroll, "background scroll clamped after trim")
	assert.Equal(t, 0, m.tabs[1].logWrapTopSkip, "background wrapTopSkip reset after trim")
}
