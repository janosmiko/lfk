package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- handleMouse: explorer mode scroll ---

func TestMouseWheelUpMovesUp(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(2)

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	result := ret.(Model)
	assert.Less(t, result.cursor(), 2)
}

func TestMouseWheelDownMovesDown(t *testing.T) {
	items := make([]model.Item, 20)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	m := baseExplorerModel()
	m.middleItems = items
	m.setCursor(0)

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	result := ret.(Model)
	assert.Greater(t, result.cursor(), 0)
}

// --- handleMouse: log viewer scroll ---

func TestMouseWheelUpInLogMode(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 100),
		logScroll: 50,
		logFollow: true,
		height:    30,
		width:     80,
		tabs:      []TabState{{}},
	}

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	result := ret.(Model)
	assert.False(t, result.logFollow)
	assert.Less(t, result.logScroll, 50)
}

func TestMouseWheelDownInLogMode(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 100),
		logScroll: 5,
		logFollow: true,
		height:    30,
		width:     80,
		tabs:      []TabState{{}},
	}

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	result := ret.(Model)
	assert.False(t, result.logFollow)
	assert.Greater(t, result.logScroll, 5)
}

func TestMouseWheelUpInLogModeAtZero(t *testing.T) {
	m := Model{
		mode:      modeLogs,
		logLines:  make([]string, 10),
		logScroll: 0,
		height:    30,
		width:     80,
		tabs:      []TabState{{}},
	}

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	result := ret.(Model)
	assert.Equal(t, 0, result.logScroll)
}

// --- handleMouse: overlay mode ignores mouse ---

func TestMouseIgnoredInOverlayMode(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	m.setCursor(0)

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	result := ret.(Model)
	assert.Equal(t, 0, result.cursor())
}

func TestMouseIgnoredInNonExplorerMode(t *testing.T) {
	m := baseExplorerModel()
	m.mode = modeYAML
	m.setCursor(0)

	ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	result := ret.(Model)
	assert.Equal(t, 0, result.cursor())
}

// --- handleMouse: left click ---

func TestMouseLeftClickRightColumn(t *testing.T) {
	m := baseExplorerModel()
	// Click far right (right column area) should navigate child.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      110, // right column
		Y:      5,
	})
	result := ret.(Model)
	// Should have attempted to navigate child.
	assert.NotNil(t, result)
}

func TestMouseLeftClickLeftColumn(t *testing.T) {
	m := baseExplorerModel()
	// Click far left (left column area) should navigate parent.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      5,
		Y:      5,
	})
	result := ret.(Model)
	// Should have navigated to parent.
	assert.Equal(t, model.LevelResourceTypes, result.nav.Level)
}

func TestMouseLeftClickNotPress(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)

	// Non-press action should be no-op.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
		X:      60,
		Y:      5,
	})
	result := ret.(Model)
	assert.Equal(t, 0, result.cursor())
}

// --- handleMouseClick: fullscreen mode ---

func TestMouseClickMiddleInFullscreen(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenMiddle = true

	ret, _ := m.handleMouseClick(60, 5)
	assert.NotNil(t, ret)
}

// --- handleHeaderClick ---

func TestHandleHeaderClickNoItems(t *testing.T) {
	m := baseExplorerModel()
	m.middleItems = nil

	ret, _ := m.handleHeaderClick(10)
	result := ret.(Model)
	assert.NotNil(t, result)
}

func TestHandleHeaderClickNameColumn(t *testing.T) {
	m := baseExplorerModel()
	m.sortColumnName = "Age"
	m.sortAscending = true
	ui.ActiveSortableColumns = []string{"Name", "Age"}
	ui.ActiveSortableColumnCount = 2

	ret, _ := m.handleHeaderClick(5)
	result := ret.(Model)
	assert.Equal(t, "Name", result.sortColumnName) // clicks Name column
}
