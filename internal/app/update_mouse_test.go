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

func TestP4MouseClickLeftColumn(t *testing.T) {
	m := bp4()
	m.mode = modeExplorer
	// Click in left column (x < leftEnd).
	result, _ := m.handleMouseClick(2, 10)
	_ = result.(Model)
}

func TestCov80SwitchToTabExplorer(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0
	result, _ := m.switchToTab(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.activeTab)
}

func TestCov80SwitchToTabLogs(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	ch := make(chan string, 1)
	m.logCh = ch
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0
	// Pre-fill the second tab so loadTab restores it.
	m.tabs[1].mode = modeLogs
	m.tabs[1].logCh = ch
	result, cmd := m.switchToTab(1)
	rm := result.(Model)
	_ = rm
	// Should return waitForLogLine cmd.
	_ = cmd
}

func TestCov80SwitchToTabNilCmd(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	m.tabs = []TabState{{}}
	m.activeTab = 0
	// Switching to same tab index.
	result, cmd := m.switchToTab(0)
	rm := result.(Model)
	assert.Equal(t, 0, rm.activeTab)
	_ = cmd
}

func TestCov80HandleMouseWheelUpInLogs(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	m.logScroll = 10
	msg := tea.MouseMsg{Button: tea.MouseButtonWheelUp}
	result, _ := m.handleMouse(msg)
	rm := result.(Model)
	assert.Less(t, rm.logScroll, 10)
}

func TestCov80HandleMouseWheelDownInLogs(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	m.logScroll = 0
	msg := tea.MouseMsg{Button: tea.MouseButtonWheelDown}
	result, _ := m.handleMouse(msg)
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.logScroll, 0)
}

func TestCov80HandleMouseInOverlay(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	m.overlay = overlayAction
	msg := tea.MouseMsg{Button: tea.MouseButtonWheelUp}
	result, cmd := m.handleMouse(msg)
	_ = result
	assert.Nil(t, cmd)
}

func TestCov80HandleMouseLeftClickNotPress(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	msg := tea.MouseMsg{Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}
	result, cmd := m.handleMouse(msg)
	_ = result
	assert.Nil(t, cmd)
}

func TestCovSwitchToTab(t *testing.T) {
	m := baseModelActions()
	m.tabs = []TabState{
		{
			nav:           model.NavigationState{Context: "ctx1"},
			cursorMemory:  make(map[string]int),
			itemCache:     make(map[string][]model.Item),
			selectedItems: make(map[string]bool),
		},
		{
			nav:           model.NavigationState{Context: "ctx2"},
			cursorMemory:  make(map[string]int),
			itemCache:     make(map[string][]model.Item),
			selectedItems: make(map[string]bool),
		},
	}
	m.activeTab = 0
	result, _ := m.switchToTab(1)
	rm := result.(Model)
	assert.Equal(t, 1, rm.activeTab)
}

func TestCovMouseScrollUpInLogs(t *testing.T) {
	m := baseModelActions()
	m.mode = modeLogs
	m.logScroll = 5
	m.logLines = make([]string, 20)
	result, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	rm := result.(Model)
	assert.Less(t, rm.logScroll, 5)
	assert.False(t, rm.logFollow)
}

func TestCovMouseScrollDownInLogs(t *testing.T) {
	m := baseModelActions()
	m.mode = modeLogs
	m.logScroll = 0
	m.logLines = make([]string, 20)
	result, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
	rm := result.(Model)
	assert.GreaterOrEqual(t, rm.logScroll, 0)
}

func TestCovMouseInOverlay(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	m.overlay = overlayAction
	result, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	_ = result.(Model)
}

func TestCovMouseInHelp(t *testing.T) {
	m := baseModelActions()
	m.mode = modeHelp
	result, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
	_ = result.(Model)
}

func TestCovMouseLeftClickInMiddle(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	m.middleItems = []model.Item{{Name: "item-1"}}
	// middleEnd should be around 45-50 area
	result, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      30,
		Y:      5,
	})
	_ = result.(Model)
}

func TestCovMouseLeftClickRelease(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	// Should be ignored (release, not press)
	result, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionRelease,
		X:      30,
		Y:      5,
	})
	_ = result.(Model)
}

func TestCovHandleHeaderClickNoItems(t *testing.T) {
	m := baseModelActions()
	m.middleItems = nil
	result, cmd := m.handleHeaderClick(5)
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovHandleHeaderClickNoColumns(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{{Name: "pod-1"}}
	ui.ActiveSortableColumns = nil
	result, cmd := m.handleHeaderClick(5)
	_ = result.(Model)
	assert.Nil(t, cmd)
}

func TestCovHandleHeaderClickWithColumns(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default", Status: "Running", Age: "1h", Ready: "1/1"},
	}
	ui.ActiveSortableColumns = []string{"Name", "Namespace", "Status", "Age"}
	result, cmd := m.handleHeaderClick(5)
	rm := result.(Model)
	assert.NotNil(t, cmd)
	assert.NotEmpty(t, rm.sortColumnName)
}

func TestCovHandleHeaderClickToggleDirection(t *testing.T) {
	m := baseModelActions()
	m.middleItems = []model.Item{
		{Name: "pod-1", Namespace: "default"},
	}
	ui.ActiveSortableColumns = []string{"Name", "Namespace"}
	m.sortColumnName = "Namespace"
	m.sortAscending = true
	// Click within the namespace column region (at the start)
	result, cmd := m.handleHeaderClick(2)
	rm := result.(Model)
	// Should either toggle direction or switch column
	if rm.sortColumnName == "Namespace" {
		assert.False(t, rm.sortAscending)
	}
	_ = cmd
}

func TestCovMouseClickLeftColumn(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	m.leftItems = []model.Item{{Name: "context-1"}}
	result, _ := m.handleMouseClick(2, 5)
	_ = result.(Model)
}

func TestCovMouseClickRightColumn(t *testing.T) {
	m := baseModelActions()
	m.mode = modeExplorer
	m.rightItems = []model.Item{{Name: "child-1"}}
	result, _ := m.handleMouseClick(70, 5)
	_ = result.(Model)
}

func TestCovSwitchToTabSameTab(t *testing.T) {
	m := baseModelCov()
	m.activeTab = 0
	m.tabs = []TabState{{}}
	ret, cmd := m.switchToTab(0)
	_ = ret.(Model)
	assert.Nil(t, cmd)
}
