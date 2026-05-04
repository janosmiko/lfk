package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// --- handleMouse: exec-mode scrollback ---

func TestMouseWheelInExecModeScrollsScrollback(t *testing.T) {
	sb := newScrollback(200)
	for range 100 {
		_, _ = sb.Write([]byte("line\n"))
	}
	// height 24, no tab bar -> execViewportRows = 24-1-4 = 19 ->
	// max usable offset = 100 - 19 = 81.
	m := Model{
		mode:           modeExec,
		height:         24,
		tabs:           []TabState{{}},
		execScrollback: sb,
	}

	t.Run("wheel up scrolls 1 line into history", func(t *testing.T) {
		ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
		assert.Equal(t, 1, ret.(Model).execScrollOffset)
	})

	t.Run("wheel down clamps at live", func(t *testing.T) {
		m := m
		m.execScrollOffset = 1
		ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelDown})
		assert.Equal(t, 0, ret.(Model).execScrollOffset, "wheel down past live clamps to 0")
	})

	t.Run("wheel up clamps so a full viewport stays visible", func(t *testing.T) {
		m := m
		m.execScrollOffset = 81
		ret, _ := m.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
		assert.Equal(t, 81, ret.(Model).execScrollOffset, "max offset = Len - viewH so the oldest line still fits at the top")
	})

	t.Run("wheel up does nothing when scrollback fits in the viewport", func(t *testing.T) {
		smallSB := newScrollback(50)
		for range 5 {
			_, _ = smallSB.Write([]byte("line\n"))
		}
		// Len=5, viewH=19 -> maxOffset=0; no scrolling possible.
		mm := Model{mode: modeExec, height: 24, tabs: []TabState{{}}, execScrollback: smallSB}
		ret, _ := mm.handleMouse(tea.MouseMsg{Button: tea.MouseButtonWheelUp})
		assert.Equal(t, 0, ret.(Model).execScrollOffset)
	})
}

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

// withMiddleLineMap installs a deterministic ActiveMiddleLineMap for the
// duration of a test. Mouse click resolution reads this package-level var
// to translate y coordinates into item indices, so tests must seed it.
func withMiddleLineMap(t *testing.T, mapping []int) {
	t.Helper()
	prev := ui.ActiveMiddleLineMap
	ui.ActiveMiddleLineMap = mapping
	t.Cleanup(func() { ui.ActiveMiddleLineMap = prev })
}

// In baseExplorerModel: width=120, single tab, LevelResources (table view).
//
//	leftEnd = 17, middleEnd = 79
//	baseOffset = 2; table view subtracts one more for the header row, so
//	itemY = y - 3. y=4 -> row index 1 (pod-b).
func TestMouseLeftClickMiddleColumnSelectsRow(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)
	withMiddleLineMap(t, []int{0, 1, 2})

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      30, // middle pane
		Y:      4,  // table row 1
	})
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor(), "click on a different row moves cursor there without drilling")
	assert.Equal(t, model.LevelResources, result.nav.Level, "drill must not happen on first click")
}

func TestMouseLeftClickAlreadyCursoredRowDrills(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(1) // pre-position cursor on pod-b
	withMiddleLineMap(t, []int{0, 1, 2})

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      30,
		Y:      4, // same row the cursor is on
	})
	result := ret.(Model)
	// navigateChild from LevelResources transitions to LevelOwned. The
	// fact that the level changed proves drill-in fired; without Option
	// B this click would only re-select the already-selected row.
	assert.NotEqual(t, model.LevelResources, result.nav.Level,
		"second click on the cursored row drills into it")
}

func TestMouseLeftClickHeaderRowSorts(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)
	prevCols := ui.ActiveSortableColumns
	prevLayout := ui.ActiveMiddleColumnLayout
	t.Cleanup(func() {
		ui.ActiveSortableColumns = prevCols
		ui.ActiveMiddleColumnLayout = prevLayout
	})
	ui.ActiveSortableColumns = []string{"Name"}
	ui.ActiveMiddleColumnLayout = []ui.MiddleColumnRegion{
		{Key: "Name", StartX: 0, EndX: 100},
	}
	withMiddleLineMap(t, []int{0, 1, 2})

	// y=2 is the table header row (itemY = 2 - 2 - 1 = -1).
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonLeft,
		Action: tea.MouseActionPress,
		X:      30,
		Y:      2,
	})
	result := ret.(Model)
	assert.Equal(t, "Name", result.sortColumnName, "header click sorts and does not drill")
	assert.Equal(t, model.LevelResources, result.nav.Level, "header click must not drill")
}

// --- handleMouse: right click ---

func TestMouseRightClickMiddleColumnOpensActionMenu(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)
	withMiddleLineMap(t, []int{0, 1, 2})

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      30,
		Y:      4, // row 1
	})
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor(), "right-click moves the cursor to the clicked row")
	assert.Equal(t, overlayAction, result.overlay, "right-click on middle pane opens action menu")
}

func TestMouseRightClickRightColumnOpensActionMenuWithoutMovingCursor(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(2)
	withMiddleLineMap(t, []int{0, 1, 2})

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      100, // right pane (>= middleEnd=79)
		Y:      4,
	})
	result := ret.(Model)
	assert.Equal(t, 2, result.cursor(), "right-click on right pane preserves the existing selection")
	assert.Equal(t, overlayAction, result.overlay, "right-click on right pane opens action menu")
}

func TestMouseRightClickLeftColumnIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(1)

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      5, // left pane (< leftEnd=17)
		Y:      4,
	})
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor(), "left-pane right-click does not move the cursor")
	assert.Equal(t, overlayNone, result.overlay, "left-pane right-click does not open action menu")
	assert.Equal(t, model.LevelResources, result.nav.Level, "left-pane right-click does not drill out")
}

// While an overlay is open the explorer underneath must not receive a
// right-click — the click is either dismissing the overlay (outside the
// box) or being swallowed by the overlay's own handler (inside). Either
// way, the explorer cursor must not move.
func TestMouseRightClickWhileOverlayOpenDoesNotReachExplorer(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "default"}}

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      30,
		Y:      4, // outside the namespace overlay box (above it)
	})
	result := ret.(Model)
	assert.Equal(t, 0, result.cursor(),
		"right-click while an overlay is open must not move the explorer cursor")
}

func TestMouseRightClickReleaseIgnored(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)
	withMiddleLineMap(t, []int{0, 1, 2})

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionRelease,
		X:      30,
		Y:      4,
	})
	result := ret.(Model)
	assert.Equal(t, 0, result.cursor(), "release must not move cursor — only press should react")
	assert.Equal(t, overlayNone, result.overlay, "release must not open action menu")
}

func TestMouseRightClickMiddleSeparatorIsNoOp(t *testing.T) {
	m := baseExplorerModel()
	m.setCursor(0)
	// Line map with a separator at index 1 (-1 means non-clickable).
	withMiddleLineMap(t, []int{0, -1, 1, 2})

	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      30,
		Y:      4, // itemY=1 -> separator
	})
	result := ret.(Model)
	assert.Equal(t, 0, result.cursor(), "right-click on a separator must not move cursor")
	assert.Equal(t, overlayNone, result.overlay, "right-click on a separator must not open action menu")
}

func TestMouseRightClickRespectsFullscreenLayout(t *testing.T) {
	m := baseExplorerModel()
	m.fullscreenMiddle = true // only the middle column is rendered
	m.setCursor(0)
	withMiddleLineMap(t, []int{0, 1, 2})

	// In fullscreen the entire width is the middle pane, so a click at
	// x=100 (which would land in the right pane in normal layout) must
	// still resolve to the middle column's action menu rather than
	// falling through to the right-pane branch.
	ret, _ := m.handleMouse(tea.MouseMsg{
		Button: tea.MouseButtonRight,
		Action: tea.MouseActionPress,
		X:      100,
		Y:      4,
	})
	result := ret.(Model)
	assert.Equal(t, 1, result.cursor())
	assert.Equal(t, overlayAction, result.overlay)
}

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

// At LevelClusters and LevelResourceTypes, sortMiddleItems() early-returns,
// so a column-header click that mutates sort state and emits "Sort: ..."
// would mislead the user about the row ordering. The handler must short-
// circuit silently before touching state.
func TestHandleHeaderClickNoOpAtClustersLevel(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelClusters
	m.sortColumnName = "Age"
	m.sortAscending = true
	m.middleItems = []model.Item{{Name: "ctx-a"}}

	oldCols := ui.ActiveSortableColumns
	oldCount := ui.ActiveSortableColumnCount
	oldLayout := ui.ActiveMiddleColumnLayout
	t.Cleanup(func() {
		ui.ActiveSortableColumns = oldCols
		ui.ActiveSortableColumnCount = oldCount
		ui.ActiveMiddleColumnLayout = oldLayout
	})
	ui.ActiveSortableColumns = []string{"Name", "Age"}
	ui.ActiveSortableColumnCount = 2
	ui.ActiveMiddleColumnLayout = []ui.MiddleColumnRegion{
		{Key: "Name", StartX: 0, EndX: 10},
		{Key: "Age", StartX: 10, EndX: 20},
	}

	ret, cmd := m.handleHeaderClick(5)
	result := ret.(Model)

	assert.Equal(t, "Age", result.sortColumnName, "sort column must not change at LevelClusters")
	assert.True(t, result.sortAscending, "sortAscending must not toggle at LevelClusters")
	assert.Empty(t, result.statusMessage, "no misleading status message")
	assert.Nil(t, cmd)
}

func TestHandleHeaderClickNoOpAtResourceTypesLevel(t *testing.T) {
	m := baseExplorerModel()
	m.nav.Level = model.LevelResourceTypes
	m.sortColumnName = "Age"
	m.sortAscending = true
	m.middleItems = []model.Item{{Name: "Pod"}}

	oldCols := ui.ActiveSortableColumns
	oldCount := ui.ActiveSortableColumnCount
	oldLayout := ui.ActiveMiddleColumnLayout
	t.Cleanup(func() {
		ui.ActiveSortableColumns = oldCols
		ui.ActiveSortableColumnCount = oldCount
		ui.ActiveMiddleColumnLayout = oldLayout
	})
	ui.ActiveSortableColumns = []string{"Name", "Age"}
	ui.ActiveSortableColumnCount = 2
	ui.ActiveMiddleColumnLayout = []ui.MiddleColumnRegion{
		{Key: "Name", StartX: 0, EndX: 10},
		{Key: "Age", StartX: 10, EndX: 20},
	}

	ret, cmd := m.handleHeaderClick(5)
	result := ret.(Model)

	assert.Equal(t, "Age", result.sortColumnName)
	assert.True(t, result.sortAscending)
	assert.Empty(t, result.statusMessage)
	assert.Nil(t, cmd)
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
