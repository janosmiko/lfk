package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

// In the log viewer the wheel must scroll the pane under the pointer: over
// the preview side panel it scrolls the structured preview, over the log
// stream it scrolls the log. Mirrors the Object Explorer per-pane routing
// (#379). width 140 -> preview panel begins at x = logW (84).

func logPreviewWheelModel() Model {
	return Model{
		mode: modeLogs,
		logView: logViewState{
			title:          "Logs",
			lines:          []string{scrollOverflowJSON},
			cursor:         0,
			previewVisible: true,
		},
		tabs:   []TabState{{}},
		width:  140,
		height: 10, // small enough to force preview overflow
	}
}

func TestLogWheel_PreviewPaneScrollsPreview(t *testing.T) {
	m := logPreviewWheelModel()
	logW, previewW := splitLogPreviewWidth(m.width)
	assert.Greater(t, previewW, 0, "preview panel must be visible at this width")

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, X: logW + 2})
	rm := ret.(Model)
	assert.Greater(t, rm.logView.previewScroll, 0,
		"wheel down over the preview panel must scroll the preview")
	assert.Equal(t, 0, rm.logView.scroll,
		"the main log must not be scrolled by a preview-pane wheel")
}

func TestLogWheel_PreviewPaneClampsAtTop(t *testing.T) {
	m := logPreviewWheelModel()
	m.logView.previewScroll = 2
	logW, _ := splitLogPreviewWidth(m.width)

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp, X: logW + 2})
	rm := ret.(Model)
	assert.Equal(t, 0, rm.logView.previewScroll,
		"wheel up must clamp the preview scroll at the top")
}

func TestLogWheel_LogPaneScrollsLog(t *testing.T) {
	m := logPreviewWheelModel()
	logW, _ := splitLogPreviewWidth(m.width)

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, X: logW - 10})
	rm := ret.(Model)
	assert.False(t, rm.logView.follow,
		"wheel over the log stream must take the main-log path (disables follow)")
	assert.Equal(t, 0, rm.logView.previewScroll,
		"wheel over the log stream must not scroll the preview")
}

func TestLogWheel_PreviewHiddenScrollsLog(t *testing.T) {
	m := logPreviewWheelModel()
	m.logView.previewVisible = false
	logW, _ := splitLogPreviewWidth(m.width)

	// Even with the pointer where the panel would be, a hidden panel means
	// the wheel scrolls the log.
	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, X: logW + 2})
	rm := ret.(Model)
	assert.False(t, rm.logView.follow, "hidden preview: wheel must scroll the log")
	assert.Equal(t, 0, rm.logView.previewScroll, "hidden preview: previewScroll stays 0")
}
