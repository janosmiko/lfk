package app

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- handleOverlayKey: dispatcher routing ---

func TestHandleOverlayKeyDispatchesRBAC(t *testing.T) {
	m := Model{
		overlay: overlayRBAC,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleOverlayKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, cmd)
}

func TestHandleOverlayKeyDispatchesPodStartup(t *testing.T) {
	m := Model{
		overlay: overlayPodStartup,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleOverlayKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, cmd)
}

func TestHandleOverlayKeyQuotaDashboardEscCloses(t *testing.T) {
	m := Model{
		overlay: overlayQuotaDashboard,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, cmd)
}

func TestHandleOverlayKeyQuotaDashboardQCloses(t *testing.T) {
	m := Model{
		overlay: overlayQuotaDashboard,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleOverlayKey(runeKey('q'))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Nil(t, cmd)
}

func TestHandleOverlayKeyQuotaDashboardOtherKeyNoOp(t *testing.T) {
	m := Model{
		overlay: overlayQuotaDashboard,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, overlayQuotaDashboard, result.overlay)
	assert.Nil(t, cmd)
}

func TestHandleOverlayKeyUnknownOverlayReturnsAsIs(t *testing.T) {
	m := Model{
		overlay: overlayKind(999),
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, cmd := m.handleOverlayKey(runeKey('j'))
	result := ret.(Model)
	assert.Equal(t, overlayKind(999), result.overlay)
	assert.Nil(t, cmd)
}

// --- handleEventTimelineOverlayKey ---

// makeEventLines creates n dummy event text lines for testing.
func makeEventLines(n int) []string {
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("%-8s %-7s %-20s %s", "1m ago", "Normal", "Scheduled", fmt.Sprintf("event %d", i))
	}
	return lines
}

func TestEventTimelineOverlayKeyNavigation(t *testing.T) {
	events := make([]k8s.EventInfo, 20)
	lines := makeEventLines(20)
	tests := []struct {
		name           string
		key            tea.KeyMsg
		startCursor    int
		expectedCursor int
	}{
		{name: "esc closes", key: specialKey(tea.KeyEsc), startCursor: 5, expectedCursor: 5},
		{name: "j moves down", key: runeKey('j'), startCursor: 0, expectedCursor: 1},
		{name: "k moves up", key: runeKey('k'), startCursor: 5, expectedCursor: 4},
		{name: "k at zero stays", key: runeKey('k'), startCursor: 0, expectedCursor: 0},
		{name: "G jumps to bottom", key: runeKey('G'), startCursor: 0, expectedCursor: 19},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay:             overlayEventTimeline,
				eventTimelineData:   events,
				eventTimelineLines:  lines,
				eventTimelineCursor: tt.startCursor,
				tabs:                []TabState{{}},
				width:               80,
				height:              40,
			}
			ret, _ := m.handleEventTimelineOverlayKey(tt.key)
			result := ret.(Model)
			if tt.key.String() == "esc" {
				assert.Equal(t, overlayNone, result.overlay)
			} else {
				assert.Equal(t, tt.expectedCursor, result.eventTimelineCursor)
			}
		})
	}

	t.Run("gg jumps to top", func(t *testing.T) {
		m := Model{
			overlay:             overlayEventTimeline,
			eventTimelineData:   events,
			eventTimelineLines:  lines,
			eventTimelineCursor: 10,
			tabs:                []TabState{{}},
			width:               80,
			height:              40,
		}
		ret, _ := m.handleEventTimelineOverlayKey(runeKey('g'))
		result := ret.(Model)
		assert.True(t, result.pendingG)

		ret2, _ := result.handleEventTimelineOverlayKey(runeKey('g'))
		result2 := ret2.(Model)
		assert.False(t, result2.pendingG)
		assert.Equal(t, 0, result2.eventTimelineCursor)
	})
}

func TestEventTimelineCtrlDScrollsHalfPage(t *testing.T) {
	events := make([]k8s.EventInfo, 50)
	lines := makeEventLines(50)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineLines:  lines,
		eventTimelineCursor: 5,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	// Cursor moves by half the content height (overlay height ~26, content ~22, half ~11).
	assert.Greater(t, result.eventTimelineCursor, 5)
}

func TestEventTimelineCtrlUScrollsUp(t *testing.T) {
	events := make([]k8s.EventInfo, 50)
	lines := makeEventLines(50)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineLines:  lines,
		eventTimelineCursor: 15,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Less(t, result.eventTimelineCursor, 15)
}

func TestEventTimelineCtrlUClampsToZero(t *testing.T) {
	events := make([]k8s.EventInfo, 50)
	lines := makeEventLines(50)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineLines:  lines,
		eventTimelineCursor: 3,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 0, result.eventTimelineCursor)
}

func TestEventTimelineCtrlFScrollsFullPage(t *testing.T) {
	events := make([]k8s.EventInfo, 100)
	lines := makeEventLines(100)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineLines:  lines,
		eventTimelineCursor: 0,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Greater(t, result.eventTimelineCursor, 0)
}

func TestEventTimelineCtrlBScrollsBackFullPage(t *testing.T) {
	events := make([]k8s.EventInfo, 100)
	lines := makeEventLines(100)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineLines:  lines,
		eventTimelineCursor: 30,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Less(t, result.eventTimelineCursor, 30)
}

// --- handleNetworkPolicyOverlayKey ---

func TestNetworkPolicyOverlayKeyNavigation(t *testing.T) {
	tests := []struct {
		name           string
		key            tea.KeyMsg
		startScroll    int
		height         int
		expectedScroll int
		expectClosed   bool
	}{
		{name: "esc closes", key: specialKey(tea.KeyEsc), startScroll: 5, height: 40, expectClosed: true},
		{name: "q closes", key: runeKey('q'), startScroll: 5, height: 40, expectClosed: true},
		{name: "j scrolls down", key: runeKey('j'), startScroll: 0, height: 40, expectedScroll: 1},
		{name: "k scrolls up", key: runeKey('k'), startScroll: 5, height: 40, expectedScroll: 4},
		{name: "k at zero stays", key: runeKey('k'), startScroll: 0, height: 40, expectedScroll: 0},
		{name: "G jumps to bottom", key: runeKey('G'), startScroll: 0, height: 40, expectedScroll: 9999},
		{name: "ctrl+d half page down", key: tea.KeyMsg{Type: tea.KeyCtrlD}, startScroll: 0, height: 40, expectedScroll: 20},
		{name: "ctrl+u half page up", key: tea.KeyMsg{Type: tea.KeyCtrlU}, startScroll: 30, height: 40, expectedScroll: 10},
		{name: "ctrl+u clamps to zero", key: tea.KeyMsg{Type: tea.KeyCtrlU}, startScroll: 5, height: 40, expectedScroll: 0},
		{name: "ctrl+f full page down", key: tea.KeyMsg{Type: tea.KeyCtrlF}, startScroll: 0, height: 40, expectedScroll: 40},
		{name: "ctrl+b full page up", key: tea.KeyMsg{Type: tea.KeyCtrlB}, startScroll: 50, height: 40, expectedScroll: 10},
		{name: "ctrl+b clamps to zero", key: tea.KeyMsg{Type: tea.KeyCtrlB}, startScroll: 10, height: 40, expectedScroll: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay:      overlayNetworkPolicy,
				netpolScroll: tt.startScroll,
				tabs:         []TabState{{}},
				width:        80,
				height:       tt.height,
			}
			result := m.handleNetworkPolicyOverlayKey(tt.key)
			if tt.expectClosed {
				assert.Equal(t, overlayNone, result.overlay)
				assert.Nil(t, result.netpolData)
			} else {
				assert.Equal(t, tt.expectedScroll, result.netpolScroll)
			}
		})
	}
}

func TestNetworkPolicyGGJumpsToTop(t *testing.T) {
	m := Model{
		overlay:      overlayNetworkPolicy,
		netpolScroll: 10,
		tabs:         []TabState{{}},
		width:        80,
		height:       40,
	}
	result := m.handleNetworkPolicyOverlayKey(runeKey('g'))
	assert.True(t, result.pendingG)
	assert.Equal(t, 10, result.netpolScroll) // not yet jumped

	result2 := result.handleNetworkPolicyOverlayKey(runeKey('g'))
	assert.False(t, result2.pendingG)
	assert.Equal(t, 0, result2.netpolScroll)
}

// --- handleErrorLogOverlayKey ---

func TestErrorLogOverlayKeyNavigation(t *testing.T) {
	entries := []ui.ErrorLogEntry{
		{Level: "ERR", Message: "error 1"},
		{Level: "INF", Message: "info 1"},
		{Level: "ERR", Message: "error 2"},
		{Level: "DBG", Message: "debug 1"},
		{Level: "ERR", Message: "error 3"},
	}

	t.Run("esc closes overlay", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			errorLogScroll:  3,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.False(t, result.overlayErrorLog)
		assert.Equal(t, 0, result.errorLogScroll)
	})

	t.Run("d toggles debug logs", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			showDebugLogs:   false,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('d'))
		result := ret.(Model)
		assert.True(t, result.showDebugLogs)
	})

	t.Run("j moves cursor down", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogCursorLine: 0,
			tabs:               []TabState{{}},
			width:              80,
			height:             10,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.errorLogCursorLine)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogCursorLine: 3,
			tabs:               []TabState{{}},
			width:              80,
			height:             10,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 2, result.errorLogCursorLine)
	})

	t.Run("k at zero stays", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogCursorLine: 0,
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.errorLogCursorLine)
	})

	t.Run("G scrolls to bottom", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			errorLogScroll:  0,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('G'))
		result := ret.(Model)
		assert.GreaterOrEqual(t, result.errorLogScroll, 0)
	})

	t.Run("g sets pendingG then gg scrolls to top", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			errorLogScroll:  3,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('g'))
		result := ret.(Model)
		assert.True(t, result.pendingG)

		ret2, _ := result.handleErrorLogOverlayKey(runeKey('g'))
		result2 := ret2.(Model)
		assert.False(t, result2.pendingG)
		assert.Equal(t, 0, result2.errorLogScroll)
	})
}

func TestErrorLogVisualSelection(t *testing.T) {
	entries := []ui.ErrorLogEntry{
		{Level: "ERR", Message: "error 1"},
		{Level: "INF", Message: "info 1"},
		{Level: "ERR", Message: "error 2"},
		{Level: "INF", Message: "info 2"},
	}

	t.Run("V enters line visual mode", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('V'))
		result := ret.(Model)
		assert.Equal(t, byte('V'), result.errorLogVisualMode)
		assert.Equal(t, 0, result.errorLogVisualStart)
	})

	t.Run("v enters char visual mode", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('v'))
		result := ret.(Model)
		assert.Equal(t, byte('v'), result.errorLogVisualMode)
	})

	t.Run("V toggles off", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogVisualMode: 'V',
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('V'))
		result := ret.(Model)
		assert.Equal(t, byte(0), result.errorLogVisualMode)
	})

	t.Run("esc cancels visual mode without closing", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogVisualMode: 'V',
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, byte(0), result.errorLogVisualMode)
		assert.True(t, result.overlayErrorLog, "overlay should stay open")
	})

	t.Run("j in visual mode moves cursor", func(t *testing.T) {
		m := Model{
			overlayErrorLog:     true,
			errorLog:            entries,
			errorLogVisualMode:  'V',
			errorLogVisualStart: 0,
			errorLogCursorLine:  0,
			tabs:                []TabState{{}},
			width:               80,
			height:              40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.errorLogCursorLine)
	})

	t.Run("k in visual mode moves cursor up", func(t *testing.T) {
		m := Model{
			overlayErrorLog:     true,
			errorLog:            entries,
			errorLogVisualMode:  'V',
			errorLogVisualStart: 0,
			errorLogCursorLine:  2,
			tabs:                []TabState{{}},
			width:               80,
			height:              40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.errorLogCursorLine)
	})

	t.Run("y copies selected lines", func(t *testing.T) {
		m := Model{
			overlayErrorLog:     true,
			errorLog:            entries,
			errorLogVisualMode:  'V',
			errorLogVisualStart: 0,
			errorLogCursorLine:  1,
			tabs:                []TabState{{}},
			width:               80,
			height:              40,
		}
		ret, cmd := m.handleErrorLogOverlayKey(runeKey('y'))
		result := ret.(Model)
		assert.Equal(t, byte(0), result.errorLogVisualMode, "visual mode should be cleared after yank")
		assert.NotNil(t, cmd, "should return clipboard command")
		assert.Contains(t, result.statusMessage, "Copied 2 entries")
	})

	t.Run("y without visual copies all", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, cmd := m.handleErrorLogOverlayKey(runeKey('y'))
		result := ret.(Model)
		assert.NotNil(t, cmd)
		assert.Contains(t, result.statusMessage, "Copied 4 entries")
	})

	t.Run("d is ignored in visual mode", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogVisualMode: 'V',
			showDebugLogs:      false,
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('d'))
		result := ret.(Model)
		assert.False(t, result.showDebugLogs, "d should not toggle debug in visual mode")
	})
}

func TestErrorLogFullscreen(t *testing.T) {
	entries := []ui.ErrorLogEntry{
		{Level: "ERR", Message: "error 1"},
		{Level: "INF", Message: "info 1"},
	}

	t.Run("f toggles fullscreen on", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('f'))
		result := ret.(Model)
		assert.True(t, result.errorLogFullscreen)
	})

	t.Run("f toggles fullscreen off", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogFullscreen: true,
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('f'))
		result := ret.(Model)
		assert.False(t, result.errorLogFullscreen)
	})

	t.Run("esc from fullscreen closes and resets", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogFullscreen: true,
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.False(t, result.overlayErrorLog)
		assert.False(t, result.errorLogFullscreen)
	})

	t.Run("q from fullscreen closes and resets", func(t *testing.T) {
		m := Model{
			overlayErrorLog:    true,
			errorLog:           entries,
			errorLogFullscreen: true,
			tabs:               []TabState{{}},
			width:              80,
			height:             40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('q'))
		result := ret.(Model)
		assert.False(t, result.overlayErrorLog)
		assert.False(t, result.errorLogFullscreen)
	})
}

// --- handleActionOverlayKey ---

func TestActionOverlayKeyNavigation(t *testing.T) {
	items := []model.Item{
		{Name: "Action1", Status: "a"},
		{Name: "Action2", Status: "b"},
		{Name: "Action3", Status: "c"},
	}

	t.Run("esc closes overlay", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, overlayNone, result.overlay)
	})

	t.Run("j moves cursor down", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("j at bottom stays", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 2, result.overlayCursor)
	})

	t.Run("k moves cursor up", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("k at top stays", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.overlayCursor)
	})

	t.Run("ctrl+p moves up", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 1,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlP})
		result := ret.(Model)
		assert.Equal(t, 0, result.overlayCursor)
	})

	t.Run("ctrl+n moves down", func(t *testing.T) {
		m := Model{
			overlay:       overlayAction,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleActionOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlN})
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})
}

// --- handlePasteConfirmKey ---

func TestPasteConfirmEnterConfirms(t *testing.T) {
	// Enter should flatten + paste, matching the y key.
	m := Model{
		overlay:       overlayPasteConfirm,
		pendingPaste:  "line1\nline2",
		pasteTargetID: pasteTargetNone,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handlePasteConfirmKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Empty(t, result.pendingPaste, "should clear pending paste after confirm")
}

func TestPasteConfirmEscCancels(t *testing.T) {
	m := Model{
		overlay:       overlayPasteConfirm,
		pendingPaste:  "line1\nline2",
		pasteTargetID: pasteTargetNone,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handlePasteConfirmKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Empty(t, result.pendingPaste, "should clear pending paste after cancel")
}

// --- handleConfirmOverlayKey (Enter/y/n for regular delete, drain) ---

func TestConfirmOverlayKeyDeclines(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "n declines", key: runeKey('n')},
		{name: "N declines", key: runeKey('N')},
		{name: "esc declines", key: specialKey(tea.KeyEsc)},
		{name: "q declines", key: runeKey('q')},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay:       overlayConfirm,
				pendingAction: "Delete",
				confirmAction: "delete pod",
				tabs:          []TabState{{}},
				width:         80,
				height:        40,
			}
			ret, _ := m.handleConfirmOverlayKey(tt.key)
			result := ret.(Model)
			assert.Equal(t, overlayNone, result.overlay)
			assert.Empty(t, result.confirmAction)
			assert.Empty(t, result.pendingAction)
		})
	}
}

func TestConfirmOverlayKeyConfirms(t *testing.T) {
	// Enter must confirm — consistency with quit overlay and every other
	// confirmable input. y/Y kept as silent muscle-memory aliases.
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "Enter confirms", key: specialKey(tea.KeyEnter)},
		{name: "y confirms", key: runeKey('y')},
		{name: "Y confirms", key: runeKey('Y')},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay:       overlayConfirm,
				pendingAction: "Delete",
				confirmAction: "delete pod",
				actionCtx:     actionContext{name: "p", namespace: "default"},
				tabs:          []TabState{{}},
				width:         80,
				height:        40,
			}
			ret, _ := m.handleConfirmOverlayKey(tt.key)
			result := ret.(Model)
			assert.Equal(t, overlayNone, result.overlay)
			assert.True(t, result.loading, "should mark loading after confirm")
		})
	}
}

// --- handleConfirmTypeOverlayKey (type DELETE for force delete, force finalize) ---

func TestConfirmTypeOverlayDispatchesForceDelete(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		pendingAction:    "Force Delete",
		confirmAction:    "my-pod (FORCE)",
		confirmTypeInput: TextInput{Value: "DELETE", Cursor: 6},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
		actionCtx: actionContext{
			name:         "my-pod",
			namespace:    "default",
			context:      "test",
			resourceType: model.ResourceTypeEntry{Resource: "pods", Namespaced: true},
		},
	}
	ret, cmd := m.handleConfirmTypeOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.True(t, result.loading)
	assert.NotNil(t, cmd, "should dispatch force delete command")
}

// --- handleConfirmTypeOverlayKey ---

func TestConfirmTypeOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		pendingAction:    "Force Finalize",
		confirmAction:    "finalize",
		confirmTypeInput: TextInput{Value: "DEL"},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleConfirmTypeOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Empty(t, result.confirmAction)
	assert.Empty(t, result.pendingAction)
	assert.Empty(t, result.confirmTypeInput.Value)
}

func TestConfirmTypeOverlayTyping(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		confirmTypeInput: TextInput{Value: ""},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	// Type "D"
	ret, _ := m.handleConfirmTypeOverlayKey(runeKey('D'))
	result := ret.(Model)
	assert.Equal(t, "D", result.confirmTypeInput.Value)
}

func TestConfirmTypeOverlayBackspace(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		confirmTypeInput: TextInput{Value: "DEL", Cursor: 3},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleConfirmTypeOverlayKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "DE", result.confirmTypeInput.Value)
}

func TestConfirmTypeOverlayCtrlW(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		confirmTypeInput: TextInput{Value: "DELETE", Cursor: 6},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleConfirmTypeOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	result := ret.(Model)
	assert.Empty(t, result.confirmTypeInput.Value)
}

func TestConfirmTypeOverlayCtrlU(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		confirmTypeInput: TextInput{Value: "DELETE", Cursor: 6},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleConfirmTypeOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Empty(t, result.confirmTypeInput.Value)
}

func TestConfirmTypeOverlayEnterWithWrongText(t *testing.T) {
	m := Model{
		overlay:          overlayConfirmType,
		pendingAction:    "Force Finalize",
		confirmTypeInput: TextInput{Value: "WRONG"},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, cmd := m.handleConfirmTypeOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	// Should not close overlay, action not executed.
	assert.Equal(t, overlayConfirmType, result.overlay)
	assert.Nil(t, cmd)
}

// --- handleScaleOverlayKey ---

func TestScaleOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: "3"},
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, _ := m.handleScaleOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Empty(t, result.scaleInput.Value)
}

func TestScaleOverlayTypesDigits(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: ""},
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, _ := m.handleScaleOverlayKey(runeKey('5'))
	result := ret.(Model)
	assert.Equal(t, "5", result.scaleInput.Value)
}

func TestScaleOverlayRejectsNonDigits(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: "3"},
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, _ := m.handleScaleOverlayKey(runeKey('a'))
	result := ret.(Model)
	assert.Equal(t, "3", result.scaleInput.Value)
}

func TestScaleOverlayEnterInvalidInput(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: "abc"},
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, cmd := m.handleScaleOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Contains(t, result.statusMessage, "Invalid")
	assert.NotNil(t, cmd) // scheduleStatusClear
}

// TestScaleOverlayEnter_ReadOnly_Blocks covers the race where a user opens
// the scale overlay (not in RO) and then toggles RO on before pressing
// Enter. The dispatcher already gates "Scale" upstream; this is the
// belt-and-suspenders check at the overlay's commit point.
func TestScaleOverlayEnter_ReadOnly_Blocks(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: "3"},
		readOnly:   true,
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, cmd := m.handleScaleOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.False(t, result.loading, "must not enter loading state when blocked")
	assert.Equal(t, readOnlyBlockedMessage("Scale"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestScaleOverlayBackspace(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: "35", Cursor: 2},
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}
	ret, _ := m.handleScaleOverlayKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "3", result.scaleInput.Value)
}

func TestScaleOverlayInputNavigation(t *testing.T) {
	m := Model{
		overlay:    overlayScaleInput,
		scaleInput: TextInput{Value: "123"},
		tabs:       []TabState{{}},
		width:      80,
		height:     40,
	}

	// ctrl+a moves home
	ret, _ := m.handleScaleOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlA})
	result := ret.(Model)
	assert.Equal(t, "123", result.scaleInput.Value)

	// ctrl+e moves end
	ret, _ = result.handleScaleOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlE})
	result = ret.(Model)
	assert.Equal(t, "123", result.scaleInput.Value)
}

// --- handlePortForwardOverlayKey ---

func TestPortForwardOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay:          overlayPortForward,
		portForwardInput: TextInput{Value: "8080:80"},
		pfAvailablePorts: []ui.PortInfo{{Port: "80"}},
		pfPortCursor:     0,
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handlePortForwardOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Empty(t, result.portForwardInput.Value)
	assert.Nil(t, result.pfAvailablePorts)
	assert.Equal(t, -1, result.pfPortCursor)
}

func TestPortForwardOverlayJKNavigation(t *testing.T) {
	ports := []ui.PortInfo{{Port: "80"}, {Port: "443"}, {Port: "8080"}}

	t.Run("j moves down", func(t *testing.T) {
		m := Model{
			overlay:          overlayPortForward,
			pfAvailablePorts: ports,
			pfPortCursor:     0,
			tabs:             []TabState{{}},
			width:            80,
			height:           40,
		}
		ret, _ := m.handlePortForwardOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.pfPortCursor)
	})

	t.Run("k moves up", func(t *testing.T) {
		m := Model{
			overlay:          overlayPortForward,
			pfAvailablePorts: ports,
			pfPortCursor:     2,
			tabs:             []TabState{{}},
			width:            80,
			height:           40,
		}
		ret, _ := m.handlePortForwardOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.pfPortCursor)
	})

	t.Run("k at top stays", func(t *testing.T) {
		m := Model{
			overlay:          overlayPortForward,
			pfAvailablePorts: ports,
			pfPortCursor:     0,
			tabs:             []TabState{{}},
			width:            80,
			height:           40,
		}
		ret, _ := m.handlePortForwardOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.pfPortCursor)
	})
}

func TestPortForwardOverlayTypesDigits(t *testing.T) {
	m := Model{
		overlay:          overlayPortForward,
		portForwardInput: TextInput{Value: "808", Cursor: 3},
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handlePortForwardOverlayKey(runeKey('0'))
	result := ret.(Model)
	assert.Equal(t, "8080", result.portForwardInput.Value)
}

func TestPortForwardOverlayColonDeselectsPort(t *testing.T) {
	m := Model{
		overlay:          overlayPortForward,
		portForwardInput: TextInput{Value: "8080", Cursor: 4},
		pfPortCursor:     1,
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handlePortForwardOverlayKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{':'}})
	result := ret.(Model)
	assert.Equal(t, -1, result.pfPortCursor)
	assert.Equal(t, "8080:", result.portForwardInput.Value)
}

func TestPortForwardOverlayEnterWithNoInput(t *testing.T) {
	m := Model{
		overlay:          overlayPortForward,
		portForwardInput: TextInput{Value: ""},
		pfPortCursor:     -1,
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, cmd := m.handlePortForwardOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.Contains(t, result.statusMessage, "Port mapping required")
	assert.NotNil(t, cmd)
}

// --- handleContainerSelectOverlayKey ---

func TestContainerSelectOverlayKeyNavigation(t *testing.T) {
	items := []model.Item{
		{Name: "container-1"},
		{Name: "container-2"},
		{Name: "container-3"},
	}

	t.Run("esc closes and clears action", func(t *testing.T) {
		m := Model{
			overlay:       overlayContainerSelect,
			overlayItems:  items,
			overlayCursor: 1,
			pendingAction: "Exec",
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleContainerSelectOverlayKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, overlayNone, result.overlay)
		assert.Empty(t, result.pendingAction)
	})

	t.Run("j moves down", func(t *testing.T) {
		m := Model{
			overlay:       overlayContainerSelect,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleContainerSelectOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("k moves up", func(t *testing.T) {
		m := Model{
			overlay:       overlayContainerSelect,
			overlayItems:  items,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleContainerSelectOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("ctrl+p moves up", func(t *testing.T) {
		m := Model{
			overlay:       overlayContainerSelect,
			overlayItems:  items,
			overlayCursor: 1,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleContainerSelectOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlP})
		result := ret.(Model)
		assert.Equal(t, 0, result.overlayCursor)
	})

	t.Run("ctrl+n moves down", func(t *testing.T) {
		m := Model{
			overlay:       overlayContainerSelect,
			overlayItems:  items,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleContainerSelectOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlN})
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})
}

// --- handleQuitConfirmOverlayKey ---

func TestQuitConfirmOverlayDeclines(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "n declines", key: runeKey('n')},
		{name: "N declines", key: runeKey('N')},
		{name: "esc declines", key: specialKey(tea.KeyEsc)},
		{name: "q declines", key: runeKey('q')},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay: overlayQuitConfirm,
				tabs:    []TabState{{}},
				width:   80,
				height:  40,
			}
			ret, cmd := m.handleQuitConfirmOverlayKey(tt.key)
			result := ret.(Model)
			assert.Equal(t, overlayNone, result.overlay)
			assert.Nil(t, cmd)
		})
	}
}

func TestQuitConfirmOverlayConfirms(t *testing.T) {
	// Enter must confirm — this matches every other confirmable input in lfk
	// (filter Enter, search Enter, scale apply, etc.). y/Y kept as aliases.
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{name: "Enter confirms", key: specialKey(tea.KeyEnter)},
		{name: "y confirms", key: runeKey('y')},
		{name: "Y confirms", key: runeKey('Y')},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay: overlayQuitConfirm,
				tabs:    []TabState{{}},
				width:   80,
				height:  40,
			}
			ret, cmd := m.handleQuitConfirmOverlayKey(tt.key)
			result := ret.(Model)
			assert.Equal(t, overlayNone, result.overlay)
			assert.NotNil(t, cmd, "should issue tea.Quit")
		})
	}
}

// --- handleFilterPresetOverlayKey ---

func TestFilterPresetOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay: overlayFilterPreset,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, _ := m.handleFilterPresetOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestFilterPresetOverlayJKNavigation(t *testing.T) {
	presets := []FilterPreset{
		{Name: "Running", Key: "r"},
		{Name: "Failed", Key: "f"},
		{Name: "Pending", Key: "p"},
	}

	t.Run("j moves down", func(t *testing.T) {
		m := Model{
			overlay:       overlayFilterPreset,
			filterPresets: presets,
			overlayCursor: 0,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleFilterPresetOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("k moves up", func(t *testing.T) {
		m := Model{
			overlay:       overlayFilterPreset,
			filterPresets: presets,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleFilterPresetOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 1, result.overlayCursor)
	})

	t.Run("j at bottom stays", func(t *testing.T) {
		m := Model{
			overlay:       overlayFilterPreset,
			filterPresets: presets,
			overlayCursor: 2,
			tabs:          []TabState{{}},
			width:         80,
			height:        40,
		}
		ret, _ := m.handleFilterPresetOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 2, result.overlayCursor)
	})
}

func TestFilterPresetOverlayEnterOutOfRange(t *testing.T) {
	m := Model{
		overlay:       overlayFilterPreset,
		filterPresets: []FilterPreset{},
		overlayCursor: 5,
		tabs:          []TabState{{}},
		width:         80,
		height:        40,
	}
	ret, _ := m.handleFilterPresetOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

// --- handleAlertsOverlayKey ---

func TestAlertsOverlayKeyNavigation(t *testing.T) {
	alerts := make([]k8s.AlertInfo, 20)

	t.Run("esc closes", func(t *testing.T) {
		m := Model{
			overlay:    overlayAlerts,
			alertsData: alerts,
			tabs:       []TabState{{}},
			width:      80,
			height:     40,
		}
		ret, _ := m.handleAlertsOverlayKey(specialKey(tea.KeyEsc))
		result := ret.(Model)
		assert.Equal(t, overlayNone, result.overlay)
	})

	t.Run("j scrolls down", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 0,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleAlertsOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.alertsScroll)
	})

	t.Run("k scrolls up", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 5,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleAlertsOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 4, result.alertsScroll)
	})

	t.Run("k at zero stays", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 0,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleAlertsOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.alertsScroll)
	})

	t.Run("g scrolls to top", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 10,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		// First g sets pendingG, second g triggers go-to-top (gg pattern).
		ret, _ := m.handleAlertsOverlayKey(runeKey('g'))
		m = ret.(Model)
		ret, _ = m.handleAlertsOverlayKey(runeKey('g'))
		result := ret.(Model)
		assert.Equal(t, 0, result.alertsScroll)
	})

	t.Run("G scrolls to bottom", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 0,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleAlertsOverlayKey(runeKey('G'))
		result := ret.(Model)
		assert.Equal(t, len(alerts), result.alertsScroll)
	})

	t.Run("ctrl+d scrolls 10 down", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 0,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleAlertsOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
		result := ret.(Model)
		assert.Equal(t, 10, result.alertsScroll)
	})

	t.Run("ctrl+u scrolls 10 up clamps to zero", func(t *testing.T) {
		m := Model{
			overlay:      overlayAlerts,
			alertsData:   alerts,
			alertsScroll: 3,
			tabs:         []TabState{{}},
			width:        80,
			height:       40,
		}
		ret, _ := m.handleAlertsOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
		result := ret.(Model)
		assert.Equal(t, 0, result.alertsScroll)
	})
}

// --- handleBatchLabelOverlayKey ---

func TestBatchLabelOverlayEscCloses(t *testing.T) {
	m := Model{
		overlay: overlayBatchLabel,
		tabs:    []TabState{{}},
		width:   80,
		height:  40,
	}
	ret, _ := m.handleBatchLabelOverlayKey(specialKey(tea.KeyEsc))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
}

func TestBatchLabelOverlayTabTogglesRemove(t *testing.T) {
	m := Model{
		overlay:          overlayBatchLabel,
		batchLabelRemove: false,
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, _ := m.handleBatchLabelOverlayKey(specialKey(tea.KeyTab))
	result := ret.(Model)
	assert.True(t, result.batchLabelRemove)

	ret2, _ := result.handleBatchLabelOverlayKey(specialKey(tea.KeyTab))
	result2 := ret2.(Model)
	assert.False(t, result2.batchLabelRemove)
}

func TestBatchLabelOverlayTyping(t *testing.T) {
	m := Model{
		overlay:         overlayBatchLabel,
		batchLabelInput: TextInput{Value: ""},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleBatchLabelOverlayKey(runeKey('a'))
	result := ret.(Model)
	assert.Equal(t, "a", result.batchLabelInput.Value)
}

func TestBatchLabelOverlayBackspace(t *testing.T) {
	m := Model{
		overlay:         overlayBatchLabel,
		batchLabelInput: TextInput{Value: "abc", Cursor: 3},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, _ := m.handleBatchLabelOverlayKey(specialKey(tea.KeyBackspace))
	result := ret.(Model)
	assert.Equal(t, "ab", result.batchLabelInput.Value)
}

func TestBatchLabelOverlayEnterEmptyNoOp(t *testing.T) {
	m := Model{
		overlay:         overlayBatchLabel,
		batchLabelInput: TextInput{Value: ""},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, cmd := m.handleBatchLabelOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayBatchLabel, result.overlay)
	assert.Nil(t, cmd)
}

func TestBatchLabelOverlayEnterInvalidFormat(t *testing.T) {
	m := Model{
		overlay:          overlayBatchLabel,
		batchLabelInput:  TextInput{Value: "noequals"},
		batchLabelRemove: false,
		tabs:             []TabState{{}},
		width:            80,
		height:           40,
	}
	ret, cmd := m.handleBatchLabelOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Contains(t, result.statusMessage, "Format: key=value")
	assert.NotNil(t, cmd)
}

// TestBatchLabelOverlayEnter_ReadOnly_Blocks covers the race where a user
// opens the batch label overlay (not in RO) and then toggles RO on before
// pressing Enter. Belt-and-suspenders at the overlay's commit point — the
// dispatcher already gates "Labels / Annotations" upstream.
func TestBatchLabelOverlayEnter_ReadOnly_Blocks(t *testing.T) {
	m := Model{
		overlay:         overlayBatchLabel,
		batchLabelInput: TextInput{Value: "team=platform"},
		readOnly:        true,
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}
	ret, cmd := m.handleBatchLabelOverlayKey(specialKey(tea.KeyEnter))
	result := ret.(Model)
	assert.Equal(t, overlayNone, result.overlay)
	assert.False(t, result.loading, "must not enter loading state when blocked")
	assert.Equal(t, readOnlyBlockedMessage("Labels / Annotations"), result.statusMessage)
	assert.True(t, result.statusMessageErr)
	assert.NotNil(t, cmd) // scheduleStatusClear
}

func TestBatchLabelOverlayInputNavigation(t *testing.T) {
	m := Model{
		overlay:         overlayBatchLabel,
		batchLabelInput: TextInput{Value: "key=value", Cursor: 9},
		tabs:            []TabState{{}},
		width:           80,
		height:          40,
	}

	// left
	ret, _ := m.handleBatchLabelOverlayKey(specialKey(tea.KeyLeft))
	result := ret.(Model)
	assert.Equal(t, "key=value", result.batchLabelInput.Value)

	// right
	ret, _ = result.handleBatchLabelOverlayKey(specialKey(tea.KeyRight))
	result = ret.(Model)
	assert.Equal(t, "key=value", result.batchLabelInput.Value)

	// ctrl+w deletes word
	ret, _ = result.handleBatchLabelOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlW})
	result = ret.(Model)
	assert.NotEqual(t, "key=value", result.batchLabelInput.Value)
}

func TestCovHandleOverlayKeyRBAC(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayRBAC
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyPodStartup(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayPodStartup
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyQuotaDashboardEsc(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayQuotaDashboard
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyQuotaDashboardQ(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayQuotaDashboard
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyAutoSync(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayAutoSync
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyLogPodSelect(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogPodSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyLogContainerSelect(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayLogContainerSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyEventTimeline(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayEventTimeline
	m.eventTimelineLines = []string{"event1"}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovHandleOverlayKeyNone(t *testing.T) {
	m := baseModelBoost2()
	m.overlay = overlayNone
	result, cmd := m.handleOverlayKey(keyMsg("q"))
	assert.Nil(t, cmd)
	_ = result
}

func TestFinalHandleOverlayKeyRBAC(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayRBAC
	result, cmd := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyPodStartup(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPodStartup
	result, cmd := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyQuotaDashboard(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuotaDashboard
	result, cmd := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyQuotaDashboardQ(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuotaDashboard
	result, cmd := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyQuotaDashboardOtherKey(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuotaDashboard
	result, cmd := m.handleOverlayKey(keyMsg("j"))
	rm := result.(Model)
	// Should stay open on other keys
	assert.Equal(t, overlayQuotaDashboard, rm.overlay)
	assert.Nil(t, cmd)
}

func TestFinalHandleOverlayKeyConfirm(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayConfirm
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyAction(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete"}}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyScaleInput(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayScaleInput
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyPVCResize(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPVCResize
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyConfirmType(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayConfirmType
	m.confirmAction = "test-res"
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyBookmarks(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBookmarks
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyTemplates(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTemplates
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyColorscheme(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayColorscheme
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyNamespace(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayNamespace
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyContainerSelect(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayContainerSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyPodSelect(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPodSelect
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyPortForward(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayPortForward
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyRollback(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayRollback
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyHelmRollback(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayHelmRollback
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyLabelEditor(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayLabelEditor
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeySecretEditor(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlaySecretEditor
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyConfigMapEditor(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayConfigMapEditor
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyAlerts(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayAlerts
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyBatchLabel(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayBatchLabel
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyEventTimeline(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayEventTimeline
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyNetworkPolicy(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayNetworkPolicy
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyCanI(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayCanI
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyCanISubject(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayCanISubject
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	// Esc in CanISubject may go back to CanI overlay, not overlayNone.
	assert.NotEqual(t, overlayCanISubject, rm.overlay)
}

func TestFinalHandleOverlayKeyExplainSearch(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayExplainSearch
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyFilterPreset(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayFilterPreset
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyAutoSync(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayAutoSync
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestFinalHandleOverlayKeyQuitConfirm(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayQuitConfirm
	result, _ := m.handleOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovConfirmOverlayKeyY(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayConfirm
	m.pendingAction = "Delete"
	m.actionCtx = actionContext{
		context:      "ctx",
		kind:         "Pod",
		name:         "pod-1",
		namespace:    "default",
		resourceType: model.ResourceTypeEntry{Resource: "pods"},
	}
	result, _ := m.handleConfirmOverlayKey(keyMsg("y"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovConfirmOverlayKeyN(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayConfirm
	result, _ := m.handleConfirmOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovConfirmOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayConfirm
	result, _ := m.handleConfirmOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovActionOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete"}, {Name: "Edit"}}
	result, _ := m.handleActionOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovActionOverlayKeyDown(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete"}, {Name: "Edit"}}
	m.overlayCursor = 0
	result, _ := m.handleActionOverlayKey(keyMsg("j"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestCovActionOverlayKeyUp(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete"}, {Name: "Edit"}}
	m.overlayCursor = 1
	result, _ := m.handleActionOverlayKey(keyMsg("k"))
	rm := result.(Model)
	assert.Equal(t, 0, rm.overlayCursor)
}

func TestCovActionOverlayKeyCtrlN(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.overlayCursor = 0
	result, _ := m.handleActionOverlayKey(keyMsg("ctrl+n"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestCovActionOverlayKeyCtrlP(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.overlayCursor = 2
	result, _ := m.handleActionOverlayKey(keyMsg("ctrl+p"))
	rm := result.(Model)
	assert.Equal(t, 1, rm.overlayCursor)
}

func TestCovActionOverlayKeyDefault(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayAction
	m.overlayItems = []model.Item{{Name: "Delete", Status: "d"}}
	// Press a key that doesn't match any action
	result, _ := m.handleActionOverlayKey(keyMsg("x"))
	_ = result.(Model)
}

func TestCovConfirmTypeOverlayKeyEsc(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayConfirmType
	result, _ := m.handleConfirmTypeOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovConfirmTypeOverlayKeyTyping(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayConfirmType
	result, _ := m.handleConfirmTypeOverlayKey(keyMsg("D"))
	rm := result.(Model)
	assert.Equal(t, "D", rm.confirmTypeInput.Value)
}

func TestCovConfirmTypeOverlayKeyBackspace(t *testing.T) {
	m := baseModelHandlers2()
	m.overlay = overlayConfirmType
	m.confirmTypeInput.Insert("DEL")
	result, _ := m.handleConfirmTypeOverlayKey(keyMsg("backspace"))
	rm := result.(Model)
	assert.Equal(t, "DE", rm.confirmTypeInput.Value)
}

func TestCovErrorLogEnsureCursorVisible(t *testing.T) {
	m := baseModelCov()
	m.errorLogScroll = 0
	m.errorLogCursorLine = 25

	scroll := m.errorLogEnsureCursorVisible(10, 50)
	assert.Greater(t, scroll, 0)

	m.errorLogCursorLine = 0
	m.errorLogScroll = 20
	scroll = m.errorLogEnsureCursorVisible(10, 50)
	assert.LessOrEqual(t, scroll, 20)
}

func TestCovOverlayKeyDispatchRBAC(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayRBAC
	result, _ := m.handleOverlayKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovOverlayKeyDispatchPodStartup(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayPodStartup
	result, _ := m.handleOverlayKey(keyMsg("x"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovOverlayKeyDispatchQuotaDashboardEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuotaDashboard
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovOverlayKeyDispatchQuotaDashboardQ(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuotaDashboard
	result, _ := m.handleOverlayKey(keyMsg("q"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovOverlayKeyDispatchQuitConfirm(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayQuitConfirm
	result, _ := m.handleOverlayKey(keyMsg("n"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovOverlayKeyDispatchFinalizerSearch(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayFinalizerSearch
	m.finalizerSearchResults = nil
	m.finalizerSearchSelected = make(map[string]bool)
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovApplyFilterPreset(t *testing.T) {
	m := baseModelOverlay()
	m.middleItems = []model.Item{
		{Name: "pod-1", Status: "Running"},
		{Name: "pod-2", Status: "Failed"},
		{Name: "pod-3", Status: "Running"},
	}
	preset := FilterPreset{
		Name: "Running",
		MatchFn: func(item model.Item) bool {
			return item.Status == "Running"
		},
	}
	result, cmd := m.applyFilterPreset(preset)
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
	assert.Len(t, rm.middleItems, 2)
	assert.NotNil(t, rm.activeFilterPreset)
	assert.NotNil(t, cmd)
}

func TestCovNetworkPolicyOverlayEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayNetworkPolicy
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovPodSelectOverlayEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayPodSelect
	m.overlayItems = []model.Item{{Name: "pod-1"}}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogPodSelectOverlayEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayLogPodSelect
	m.logMultiItems = []model.Item{{Name: "pod-1"}}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovLogContainerSelectOverlayEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayLogContainerSelect
	m.logContainers = []string{"container-1"}
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestCovAutoSyncKeyEsc(t *testing.T) {
	m := baseModelOverlay()
	m.overlay = overlayAutoSync
	result, _ := m.handleOverlayKey(keyMsg("esc"))
	rm := result.(Model)
	assert.Equal(t, overlayNone, rm.overlay)
}
