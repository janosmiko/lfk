package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
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

// scrollOverflowJSON in the app package mirrors the one in internal/ui:
// a JSON line whose pretty-printed body easily exceeds a small height so
// scroll math can be exercised deterministically.
const scrollOverflowJSON = `{"a":"1","b":"2","c":"3","d":"4","e":"5","f":"6","g":"7","h":"8"}`

func TestLogKeyJCapitalScrollsPreviewDown(t *testing.T) {
	m := Model{
		mode:              modeLogs,
		logLines:          []string{scrollOverflowJSON},
		logCursor:         0,
		logPreviewVisible: true,
		tabs:              []TabState{{}},
		width:             140,
		height:            10, // small enough to force preview overflow
		logTitle:          "Logs",
	}
	ret, _ := m.handleLogKey(runeKey('J'))
	scrolled := ret.(Model)
	assert.Equal(t, 1, scrolled.logPreviewScroll, "J should advance preview scroll by 1")
}

func TestLogKeyKCapitalScrollsPreviewUp(t *testing.T) {
	m := Model{
		mode:              modeLogs,
		logLines:          []string{scrollOverflowJSON},
		logCursor:         0,
		logPreviewVisible: true,
		logPreviewScroll:  3,
		tabs:              []TabState{{}},
		width:             140,
		height:            10,
		logTitle:          "Logs",
	}
	ret, _ := m.handleLogKey(runeKey('K'))
	scrolled := ret.(Model)
	assert.Equal(t, 2, scrolled.logPreviewScroll, "K should rewind preview scroll by 1")
}

func TestLogKeyJCapitalNoOpWhenPreviewHidden(t *testing.T) {
	// J must not consume the key when preview is off. handleLogActionKey
	// returning ok=false means the dispatcher leaves m unchanged at the
	// outer handleLogKey level. We confirm scroll stays 0.
	m := Model{
		mode:              modeLogs,
		logLines:          []string{scrollOverflowJSON},
		logCursor:         0,
		logPreviewVisible: false,
		tabs:              []TabState{{}},
		width:             140,
		height:            10,
		logTitle:          "Logs",
	}
	ret, _ := m.handleLogKey(runeKey('J'))
	after := ret.(Model)
	assert.Equal(t, 0, after.logPreviewScroll, "J must not scroll preview when preview is hidden")
}

func TestLogKeyJCapitalClampsAtMax(t *testing.T) {
	m := Model{
		mode:              modeLogs,
		logLines:          []string{scrollOverflowJSON},
		logCursor:         0,
		logPreviewVisible: true,
		tabs:              []TabState{{}},
		width:             140,
		height:            10,
		logTitle:          "Logs",
	}
	// Press J many more times than the body has rows.
	for range 100 {
		ret, _ := m.handleLogKey(runeKey('J'))
		m = ret.(Model)
	}
	_, previewW := splitLogPreviewWidth(m.width)
	// Mirror the handler: maxScroll is computed from the inner content
	// height (logContentHeight), and LogPreviewMaxScroll's height arg is
	// the outer panel size, so add 2 for the border.
	wantMax := ui.LogPreviewMaxScroll(m.logPreviewLine(), previewW, m.logContentHeight()+2)
	assert.Equal(t, wantMax, m.logPreviewScroll, "J spam must clamp at LogPreviewMaxScroll")
}

func TestLogKeyKCapitalClampsAtZero(t *testing.T) {
	m := Model{
		mode:              modeLogs,
		logLines:          []string{scrollOverflowJSON},
		logCursor:         0,
		logPreviewVisible: true,
		logPreviewScroll:  0,
		tabs:              []TabState{{}},
		width:             140,
		height:            10,
		logTitle:          "Logs",
	}
	ret, _ := m.handleLogKey(runeKey('K'))
	after := ret.(Model)
	assert.Equal(t, 0, after.logPreviewScroll, "K at scroll 0 must not go negative")
}

func TestEnsureLogCursorVisibleResetsPreviewScroll(t *testing.T) {
	// Whenever the cursor moves the previewed line changes, so any prior
	// scroll offset is stale and must be cleared. ensureLogCursorVisible
	// is the chokepoint every cursor handler funnels through.
	m := Model{
		logLines:         []string{"a", "b", "c"},
		logCursor:        2,
		logPreviewScroll: 5,
		width:            120,
		height:           20,
	}
	m.ensureLogCursorVisible()
	assert.Equal(t, 0, m.logPreviewScroll, "ensureLogCursorVisible must reset preview scroll")
}

func TestScrollToMaxRevealsLastBodyRowInRenderedView(t *testing.T) {
	// Regression: the J handler computes maxScroll from m.logContentHeight()
	// because m.logViewHeight() depends on View()'s mutation of m.height
	// for the app title bar (and optional tab bar). Using the wrong height
	// makes maxScroll 1-2 short and hides the last body rows from the user.
	//
	// This test mirrors the user-visible flow: build a body that overflows,
	// spam J to reach max, then assert the last body line appears in the
	// rendered viewLogs output. View() reduces m.height by 1 before
	// dispatching, so we mimic that reduction in the test setup so viewLogs
	// sees the same height the real app pipeline would give it.
	const sentinel = "END_OF_BODY_MARKER"
	// Many fields so body line count exceeds the panel height regardless
	// of which side of the off-by-one bug we're on.
	jsonLine := `{"a":"1","b":"2","c":"3","d":"4","e":"5","f":"6","g":"7","h":"8","i":"9","j":"10","k":"11","l":"12","m":"13","n":"14","o":"15","p":"16","q":"17","r":"18","s":"19","t":"20","u":"21","v":"22","w":"23","x":"24","y":"25","z_last_field":"` + sentinel + `"}`
	terminalH := 16
	m := Model{
		mode:              modeLogs,
		logLines:          []string{jsonLine},
		logCursor:         0,
		logPreviewVisible: true,
		tabs:              []TabState{{}}, // single tab → no tab bar
		width:             140,
		height:            terminalH, // Update-context: untouched terminal height
		logTitle:          "Logs",
	}
	// Spam J more times than there could possibly be body rows so we land
	// at whatever maxScroll the handler considers final.
	for range 100 {
		ret, _ := m.handleLogKey(runeKey('J'))
		m = ret.(Model)
	}
	// Render viewLogs the same way View() would: with m.height reduced by
	// 1 for the app title bar.
	m.height = terminalH - 1
	out := m.viewLogs()
	assert.Contains(t, out, sentinel,
		"after spamming J the last body row must be visible in viewLogs output; "+
			"if maxScroll is computed against the wrong height the handler stops "+
			"short and the user sees the bottom of the body clipped off-screen")
}

func TestLogKeyJLowercaseResetsPreviewScroll(t *testing.T) {
	// Behavioral guarantee that the scroll-reset path is wired up: pressing
	// lowercase j (cursor down) must reset any preview scroll because the
	// cursor lands on a different line whose preview is unrelated.
	m := Model{
		mode:              modeLogs,
		logLines:          []string{"first line", "second line"},
		logCursor:         0,
		logPreviewVisible: true,
		logPreviewScroll:  4,
		tabs:              []TabState{{}},
		width:             140,
		height:            20,
		logTitle:          "Logs",
	}
	ret, _ := m.handleLogKey(runeKey('j'))
	after := ret.(Model)
	assert.Equal(t, 0, after.logPreviewScroll, "lowercase j must reset preview scroll via ensureLogCursorVisible")
}
