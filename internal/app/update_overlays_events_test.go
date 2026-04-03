package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/k8s"
)

func newEventModel(numEvents int) Model {
	events := make([]k8s.EventInfo, numEvents)
	for i := range events {
		events[i] = k8s.EventInfo{
			Type:    "Normal",
			Reason:  "Scheduled",
			Message: "event message",
		}
	}
	m := Model{
		overlay:           overlayEventTimeline,
		eventTimelineData: events,
		tabs:              []TabState{{}},
		width:             80,
		height:            40,
	}
	m.eventTimelineLines = m.buildEventTimelineLines()
	return m
}

func TestBuildEventTimelineLines(t *testing.T) {
	m := newEventModel(5)
	assert.Len(t, m.eventTimelineLines, 5)
	for _, line := range m.eventTimelineLines {
		assert.Contains(t, line, "Normal")
		assert.Contains(t, line, "Scheduled")
		assert.Contains(t, line, "event message")
	}
}

func TestBuildEventTimelineLinesEmpty(t *testing.T) {
	m := Model{eventTimelineData: nil}
	lines := m.buildEventTimelineLines()
	assert.Empty(t, lines)
}

func TestEventContentHeight(t *testing.T) {
	m := newEventModel(10)

	// Overlay mode.
	h := m.eventContentHeight()
	assert.Greater(t, h, 0)

	// Fullscreen mode (via modeEventViewer).
	m.mode = modeEventViewer
	fh := m.eventContentHeight()
	assert.Greater(t, fh, 0, "fullscreen should have positive height")
}

func TestEnsureEventCursorVisible(t *testing.T) {
	m := newEventModel(100)
	m.eventTimelineCursor = 50
	m.eventTimelineScroll = 0
	m.ensureEventCursorVisible()
	assert.Greater(t, m.eventTimelineScroll, 0, "scroll should adjust to show cursor")
}

func TestEventTimelineVisualModeToggle(t *testing.T) {
	m := newEventModel(10)

	// Enter char visual mode.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('v'))
	result := ret.(Model)
	assert.Equal(t, byte('v'), result.eventTimelineVisualMode)
	assert.Equal(t, 0, result.eventTimelineVisualStart)

	// Cancel with esc.
	ret2, _ := result.handleEventTimelineVisualKey(tea.KeyMsg{Type: tea.KeyEsc})
	result2 := ret2.(Model)
	assert.Equal(t, byte(0), result2.eventTimelineVisualMode)
}

func TestEventTimelineLineVisualMode(t *testing.T) {
	m := newEventModel(10)

	ret, _ := m.handleEventTimelineOverlayKey(runeKey('V'))
	result := ret.(Model)
	assert.Equal(t, byte('V'), result.eventTimelineVisualMode)
}

func TestEventTimelineBlockVisualMode(t *testing.T) {
	m := newEventModel(10)

	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlV})
	result := ret.(Model)
	assert.Equal(t, byte('B'), result.eventTimelineVisualMode)
}

func TestEventTimelineVisualModeSwitching(t *testing.T) {
	m := newEventModel(10)

	// Enter line visual mode.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('V'))
	result := ret.(Model)
	require.Equal(t, byte('V'), result.eventTimelineVisualMode)

	// Switch to char visual mode.
	ret2, _ := result.handleEventTimelineVisualKey(runeKey('v'))
	result2 := ret2.(Model)
	assert.Equal(t, byte('v'), result2.eventTimelineVisualMode)

	// Toggle off.
	ret3, _ := result2.handleEventTimelineVisualKey(runeKey('v'))
	result3 := ret3.(Model)
	assert.Equal(t, byte(0), result3.eventTimelineVisualMode)
}

func TestEventTimelineVisualCopy(t *testing.T) {
	m := newEventModel(10)

	// Enter line visual mode.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('V'))
	result := ret.(Model)

	// Move down to select 3 lines.
	ret2, _ := result.handleEventTimelineVisualKey(runeKey('j'))
	result2 := ret2.(Model)
	ret3, _ := result2.handleEventTimelineVisualKey(runeKey('j'))
	result3 := ret3.(Model)

	// Copy.
	ret4, cmd := result3.handleEventTimelineVisualKey(runeKey('y'))
	result4 := ret4.(Model)
	assert.Equal(t, byte(0), result4.eventTimelineVisualMode, "visual mode should be cleared after copy")
	assert.NotNil(t, cmd, "should return clipboard command")
	assert.Contains(t, result4.statusMessage, "3 line")
}

func TestEventTimelineSearchMode(t *testing.T) {
	m := newEventModel(10)

	// Enter search mode.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('/'))
	result := ret.(Model)
	assert.True(t, result.eventTimelineSearchActive)

	// Type query.
	ret2, _ := result.handleEventTimelineSearchKey(runeKey('e'))
	result2 := ret2.(Model)
	assert.Equal(t, "e", result2.eventTimelineSearchInput.Value)

	// Press enter to apply.
	ret3, _ := result2.handleEventTimelineSearchKey(tea.KeyMsg{Type: tea.KeyEnter})
	result3 := ret3.(Model)
	assert.False(t, result3.eventTimelineSearchActive)
	assert.Equal(t, "e", result3.eventTimelineSearchQuery)
}

func TestEventTimelineSearchEsc(t *testing.T) {
	m := newEventModel(10)
	m.eventTimelineSearchActive = true
	m.eventTimelineSearchInput.Insert("test")

	ret, _ := m.handleEventTimelineSearchKey(tea.KeyMsg{Type: tea.KeyEsc})
	result := ret.(Model)
	assert.False(t, result.eventTimelineSearchActive)
}

func TestEventTimelineSearchBackspace(t *testing.T) {
	m := newEventModel(10)
	m.eventTimelineSearchActive = true
	m.eventTimelineSearchInput.Insert("abc")

	ret, _ := m.handleEventTimelineSearchKey(tea.KeyMsg{Type: tea.KeyBackspace})
	result := ret.(Model)
	assert.Equal(t, "ab", result.eventTimelineSearchInput.Value)
}

func TestEventTimelineFullscreenToggle(t *testing.T) {
	m := newEventModel(10)

	// Pressing f switches to modeEventViewer and clears overlay.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('f'))
	result := ret.(Model)
	assert.Equal(t, modeEventViewer, result.mode)
	assert.Equal(t, overlayNone, result.overlay)

	// Pressing f in fullscreen mode goes back to overlay.
	ret2, _ := result.handleEventViewerModeKey(runeKey('f'))
	result2 := ret2.(Model)
	assert.Equal(t, modeExplorer, result2.mode)
	assert.Equal(t, overlayEventTimeline, result2.overlay)
}

func TestEventTimelineWrapToggle(t *testing.T) {
	m := newEventModel(10)

	ret, _ := m.handleEventTimelineOverlayKey(runeKey('>'))
	result := ret.(Model)
	assert.True(t, result.eventTimelineWrap)

	ret2, _ := result.handleEventTimelineOverlayKey(runeKey('>'))
	result2 := ret2.(Model)
	assert.False(t, result2.eventTimelineWrap)
}

func TestEventTimelineCursorMovementLR(t *testing.T) {
	m := newEventModel(10)

	// Move right.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('l'))
	result := ret.(Model)
	assert.Equal(t, 1, result.eventTimelineCursorCol)

	// Move left.
	ret2, _ := result.handleEventTimelineOverlayKey(runeKey('h'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.eventTimelineCursorCol)

	// Can't go below 0.
	ret3, _ := result2.handleEventTimelineOverlayKey(runeKey('h'))
	result3 := ret3.(Model)
	assert.Equal(t, 0, result3.eventTimelineCursorCol)
}

func TestEventTimelineDollarEndOfLine(t *testing.T) {
	m := newEventModel(10)

	ret, _ := m.handleEventTimelineOverlayKey(runeKey('$'))
	result := ret.(Model)
	lineLen := len([]rune(m.eventTimelineLines[0]))
	assert.Equal(t, lineLen-1, result.eventTimelineCursorCol)
}

func TestEventTimelineCaretFirstNonWhitespace(t *testing.T) {
	m := newEventModel(10)
	m.eventTimelineCursorCol = 10

	ret, _ := m.handleEventTimelineOverlayKey(runeKey('^'))
	result := ret.(Model)
	assert.LessOrEqual(t, result.eventTimelineCursorCol, 10)
}

func TestEventTimelineYankSingleLine(t *testing.T) {
	m := newEventModel(10)

	ret, cmd := m.handleEventTimelineOverlayKey(runeKey('y'))
	result := ret.(Model)
	assert.NotNil(t, cmd)
	assert.Contains(t, result.statusMessage, "1 line")
}

func TestEventTimelineSearchNextPrev(t *testing.T) {
	m := newEventModel(10)
	m.eventTimelineSearchQuery = "event"

	// n: find next.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('n'))
	result := ret.(Model)
	assert.Equal(t, 1, result.eventTimelineCursor) // wraps to next match

	// N: find previous.
	ret2, _ := result.handleEventTimelineOverlayKey(runeKey('N'))
	result2 := ret2.(Model)
	assert.Equal(t, 0, result2.eventTimelineCursor)
}

func TestEventTimelineEscClearsSearchFirst(t *testing.T) {
	m := newEventModel(10)
	m.eventTimelineSearchQuery = "test"

	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyEsc})
	result := ret.(Model)
	assert.Equal(t, "", result.eventTimelineSearchQuery)
	assert.Equal(t, overlayEventTimeline, result.overlay, "should not close overlay yet")
}

func TestEventTimelineQClosesOverlay(t *testing.T) {
	m := newEventModel(10)

	ret, _ := m.handleEventTimelineOverlayKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestEventTimelineDigitBufferG(t *testing.T) {
	m := newEventModel(20)

	// Type "10" then G to jump to line 10.
	ret, _ := m.handleEventTimelineOverlayKey(runeKey('1'))
	result := ret.(Model)
	ret2, _ := result.handleEventTimelineOverlayKey(runeKey('0'))
	result2 := ret2.(Model)
	ret3, _ := result2.handleEventTimelineOverlayKey(runeKey('G'))
	result3 := ret3.(Model)
	assert.Equal(t, 9, result3.eventTimelineCursor) // 0-indexed
}

func TestFindNextEventMatchWraps(t *testing.T) {
	m := newEventModel(5)
	m.eventTimelineSearchQuery = "event"
	m.eventTimelineCursor = 4 // last line

	m.findNextEventMatch(true) // forward should wrap to 0
	assert.Equal(t, 0, m.eventTimelineCursor)
}

func TestFindNextEventMatchNotFound(t *testing.T) {
	m := newEventModel(5)
	m.eventTimelineSearchQuery = "nonexistent_query_xyz"

	m.findNextEventMatch(true)
	// Cursor should not change.
	assert.Equal(t, 0, m.eventTimelineCursor)
}

func TestCovAlertsKeyEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayAlerts
	m.alertsData = []k8s.AlertInfo{{Name: "alert1"}, {Name: "alert2"}}
	result, _ := m.handleAlertsOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovAlertsKeyDown(t *testing.T) {
	m := baseModelOverlay()
	m.alertsData = []k8s.AlertInfo{{Name: "alert1"}, {Name: "alert2"}}
	m.alertsScroll = 0
	result, _ := m.handleAlertsOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.alertsScroll)
}

func TestCovAlertsKeyUp(t *testing.T) {
	m := baseModelOverlay()
	m.alertsData = []k8s.AlertInfo{{Name: "alert1"}, {Name: "alert2"}}
	m.alertsScroll = 1
	result, _ := m.handleAlertsOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.alertsScroll)
}

func TestCovAlertsKeyUpAtZero(t *testing.T) {
	m := baseModelOverlay()
	m.alertsScroll = 0
	result, _ := m.handleAlertsOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.alertsScroll)
}

func TestCovAlertsKeyGG(t *testing.T) {
	m := baseModelOverlay()
	m.alertsScroll = 5
	m.alertsData = []k8s.AlertInfo{{Name: "a"}}
	result, _ := m.handleAlertsOverlayKey(keyMsg("g"))
	rm := result.(Model)
	assert.True(t, rm.pendingG)
	result, _ = rm.handleAlertsOverlayKey(keyMsg("g"))
	rm = result.(Model)
	assert.Equal(t, 0, rm.alertsScroll)
}

func TestCovAlertsKeyBigG(t *testing.T) {
	m := baseModelOverlay()
	m.alertsData = []k8s.AlertInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	result, _ := m.handleAlertsOverlayKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 3, rm.alertsScroll) // len(alertsData)
}

func TestCovAlertsKeyBigGWithLineInput(t *testing.T) {
	m := baseModelOverlay()
	m.alertsData = []k8s.AlertInfo{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.alertsLineInput = "2"
	result, _ := m.handleAlertsOverlayKey(keyMsg("G"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.alertsScroll) // 2-1=1
}

func TestCovAlertsKeyDigit(t *testing.T) {
	m := baseModelOverlay()
	result, _ := m.handleAlertsOverlayKey(keyMsg("5"))
	rm := result.(Model)
	assert.Equal(t, "5", rm.alertsLineInput)
}

func TestCovAlertsKeyZeroInInput(t *testing.T) {
	m := baseModelOverlay()
	m.alertsLineInput = "1"
	result, _ := m.handleAlertsOverlayKey(keyMsg("0"))
	rm := result.(Model)
	assert.Equal(t, "10", rm.alertsLineInput)
}

func TestCovAlertsKeyCtrlD(t *testing.T) {
	m := baseModelOverlay()
	m.alertsScroll = 0
	result, _ := m.handleAlertsOverlayKey(keyMsg("ctrl+d"))
	rm := result.(Model)
	assert.Equal(t, 10, rm.alertsScroll)
}

func TestCovAlertsKeyCtrlU(t *testing.T) {
	m := baseModelOverlay()
	m.alertsScroll = 15
	result, _ := m.handleAlertsOverlayKey(keyMsg("ctrl+u"))
	rm := result.(Model)
	assert.Equal(t, 5, rm.alertsScroll)
}

func TestCovAlertsKeyCtrlF(t *testing.T) {
	m := baseModelOverlay()
	m.alertsScroll = 0
	result, _ := m.handleAlertsOverlayKey(keyMsg("ctrl+f"))
	rm := result.(Model)
	assert.Equal(t, 20, rm.alertsScroll)
}

func TestCovAlertsKeyCtrlB(t *testing.T) {
	m := baseModelOverlay()
	m.alertsScroll = 25
	result, _ := m.handleAlertsOverlayKey(keyMsg("ctrl+b"))
	rm := result.(Model)
	assert.Equal(t, 5, rm.alertsScroll)
}
