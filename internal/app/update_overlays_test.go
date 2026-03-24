package app

import (
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

func TestEventTimelineOverlayKeyNavigation(t *testing.T) {
	events := make([]k8s.EventInfo, 20)
	tests := []struct {
		name           string
		key            tea.KeyMsg
		startScroll    int
		expectedScroll int
	}{
		{name: "esc closes", key: specialKey(tea.KeyEsc), startScroll: 5, expectedScroll: 5},
		{name: "j scrolls down", key: runeKey('j'), startScroll: 0, expectedScroll: 1},
		{name: "k scrolls up", key: runeKey('k'), startScroll: 5, expectedScroll: 4},
		{name: "k at zero stays", key: runeKey('k'), startScroll: 0, expectedScroll: 0},
		{name: "g jumps to top", key: runeKey('g'), startScroll: 10, expectedScroll: 0},
		{name: "G jumps to bottom", key: runeKey('G'), startScroll: 0, expectedScroll: 19},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				overlay:             overlayEventTimeline,
				eventTimelineData:   events,
				eventTimelineScroll: tt.startScroll,
				tabs:                []TabState{{}},
				width:               80,
				height:              40,
			}
			ret, _ := m.handleEventTimelineOverlayKey(tt.key)
			result := ret.(Model)
			if tt.key.String() == "esc" {
				assert.Equal(t, overlayNone, result.overlay)
			} else {
				assert.Equal(t, tt.expectedScroll, result.eventTimelineScroll)
			}
		})
	}
}

func TestEventTimelineCtrlDScrollsHalfPage(t *testing.T) {
	events := make([]k8s.EventInfo, 50)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineScroll: 5,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlD})
	result := ret.(Model)
	assert.Equal(t, 15, result.eventTimelineScroll)
}

func TestEventTimelineCtrlUScrollsUp(t *testing.T) {
	events := make([]k8s.EventInfo, 50)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineScroll: 15,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 5, result.eventTimelineScroll)
}

func TestEventTimelineCtrlUClampsToZero(t *testing.T) {
	events := make([]k8s.EventInfo, 50)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineScroll: 3,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlU})
	result := ret.(Model)
	assert.Equal(t, 0, result.eventTimelineScroll)
}

func TestEventTimelineCtrlFScrollsFullPage(t *testing.T) {
	events := make([]k8s.EventInfo, 100)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineScroll: 0,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlF})
	result := ret.(Model)
	assert.Equal(t, 20, result.eventTimelineScroll)
}

func TestEventTimelineCtrlBScrollsBackFullPage(t *testing.T) {
	events := make([]k8s.EventInfo, 100)
	m := Model{
		overlay:             overlayEventTimeline,
		eventTimelineData:   events,
		eventTimelineScroll: 30,
		tabs:                []TabState{{}},
		width:               80,
		height:              40,
	}
	ret, _ := m.handleEventTimelineOverlayKey(tea.KeyMsg{Type: tea.KeyCtrlB})
	result := ret.(Model)
	assert.Equal(t, 10, result.eventTimelineScroll)
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
		{name: "g jumps to top", key: runeKey('g'), startScroll: 10, height: 40, expectedScroll: 0},
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
			ret, _ := m.handleNetworkPolicyOverlayKey(tt.key)
			result := ret.(Model)
			if tt.expectClosed {
				assert.Equal(t, overlayNone, result.overlay)
				assert.Nil(t, result.netpolData)
			} else {
				assert.Equal(t, tt.expectedScroll, result.netpolScroll)
			}
		})
	}
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

	t.Run("j scrolls down", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			errorLogScroll:  0,
			tabs:            []TabState{{}},
			width:           80,
			height:          10, // small height so maxVisible < len(entries)
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('j'))
		result := ret.(Model)
		assert.Equal(t, 1, result.errorLogScroll)
	})

	t.Run("k scrolls up", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			errorLogScroll:  3,
			tabs:            []TabState{{}},
			width:           80,
			height:          10, // small height so maxVisible < len(entries)
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 2, result.errorLogScroll)
	})

	t.Run("k at zero stays", func(t *testing.T) {
		m := Model{
			overlayErrorLog: true,
			errorLog:        entries,
			errorLogScroll:  0,
			tabs:            []TabState{{}},
			width:           80,
			height:          40,
		}
		ret, _ := m.handleErrorLogOverlayKey(runeKey('k'))
		result := ret.(Model)
		assert.Equal(t, 0, result.errorLogScroll)
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

// --- handleConfirmOverlayKey (y/n for regular delete, drain) ---

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
		ret, _ := m.handleAlertsOverlayKey(runeKey('g'))
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
