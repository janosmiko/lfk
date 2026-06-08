package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// In the Object Explorer the wheel must scroll the pane under the pointer:
// over the right (preview) pane it scrolls the YAML, over the left/middle
// panes it moves the tree cursor — matching the main explorer's per-pane
// routing (#379). A short window forces the preview to be scrollable.

// openObjectExplorerOnStatus opens the explorer with the cursor on the
// "status" node (a multi-line subtree) and a short window so the preview
// pane can scroll.
func openObjectExplorerOnStatus(t *testing.T) Model {
	t.Helper()
	m := objectExplorerModel(t)
	result, _ := m.openObjectExplorer()
	m = result.(Model)
	m.height = 8 // previewPaneHeight = height-5 = 3, well under the YAML length
	// Root keys are sorted [apiVersion, kind, metadata, status]; status is last.
	m.objectExplorerView.cursor = len(m.objectExplorerView.level) - 1
	require.Greater(t, len(m.selectedNodeYAML()), 0, "status node must have a preview")
	return m
}

func TestObjectExplorerWheel_RightPaneScrollsPreview(t *testing.T) {
	m := openObjectExplorerOnStatus(t)
	cursorBefore := m.objectExplorerView.cursor
	x := m.objectExplorerRightPaneStart() + 2 // inside the preview pane

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, X: x})
	rm := ret.(Model)
	assert.Greater(t, rm.objectExplorerView.previewScroll, 0,
		"wheel down over the right pane must scroll the preview")
	assert.Equal(t, cursorBefore, rm.objectExplorerView.cursor,
		"wheel over the right pane must not move the tree cursor")
}

func TestObjectExplorerWheel_RightPaneClampsAtTop(t *testing.T) {
	m := openObjectExplorerOnStatus(t)
	m.objectExplorerView.previewScroll = 2
	x := m.objectExplorerRightPaneStart() + 2

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp, X: x})
	rm := ret.(Model)
	assert.Equal(t, 0, rm.objectExplorerView.previewScroll,
		"wheel up must clamp the preview scroll at the top")
}

func TestObjectExplorerWheel_MiddlePaneMovesCursor(t *testing.T) {
	m := openObjectExplorerOnStatus(t)
	m.objectExplorerView.cursor = 0
	m.objectExplorerView.previewScroll = 5
	x := m.objectExplorerRightPaneStart() - 10 // inside the middle pane

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown, X: x})
	rm := ret.(Model)
	assert.Greater(t, rm.objectExplorerView.cursor, 0,
		"wheel down over the middle pane must move the tree cursor")
	assert.Equal(t, 0, rm.objectExplorerView.previewScroll,
		"moving the cursor must reset the preview scroll")
}
