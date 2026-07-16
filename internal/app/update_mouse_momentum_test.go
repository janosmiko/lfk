package app

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// Issue #524: trackpad momentum keeps emitting wheel ticks after the gesture
// ends. Those queued ticks must not "play out" on whatever list is under the
// pointer once the burst's target changes, the cursor hits a boundary, or the
// user navigates away.

func wheel(m Model, button tea.MouseButton, x int) Model {
	mdl, _ := m.handleMouse(tea.MouseMsg{Button: button, X: x})
	return mdl.(Model)
}

func longListModel(n int) Model {
	m := baseExplorerModel()
	items := make([]model.Item, n)
	for i := range items {
		items[i] = model.Item{Name: "pod", Kind: "Pod"}
	}
	m.middleItems = items
	m.setCursor(0)
	m.previewYAML = strings.Repeat("field: value\n", 200)
	return m
}

// Momentum from the right (preview) pane must not scroll the middle list when
// the pointer moves onto it mid-burst (the reporter's repro: swipe the detail
// pane, then move the mouse over the main list).
func TestWheelMomentumDoesNotLeakAcrossPanes(t *testing.T) {
	m := longListModel(30)
	m.previewScroll = 10

	// Burst tick 1 over the right pane (X=100 >= middleEnd=75) scrolls preview.
	m = wheel(m, tea.MouseButtonWheelUp, 100)
	require.Less(t, m.previewScroll, 10, "wheel over the right pane scrolls the preview")
	cursorBefore := m.cursor()

	// Same burst, pointer now over the middle list: the momentum tail must be
	// dropped, not moved onto the list cursor.
	m = wheel(m, tea.MouseButtonWheelDown, 30)
	assert.Equal(t, cursorBefore, m.cursor(),
		"momentum from the preview pane must not scroll the list once the pointer moves")
}

// Reaching the bottom of the list empties the momentum queue (user rule 1).
func TestWheelMomentumStopsAtListBottom(t *testing.T) {
	m := longListModel(4)

	// Fling down through the whole list within a single burst.
	for range 10 {
		m = wheel(m, tea.MouseButtonWheelDown, 30)
	}
	require.Equal(t, len(m.middleItems)-1, m.cursor(), "cursor reaches the last item")
	assert.True(t, m.wheel.dead, "reaching the list bottom empties the momentum queue")
}

// Reaching the top of the list empties the momentum queue (user rule 1).
func TestWheelMomentumStopsAtListTop(t *testing.T) {
	m := longListModel(4)
	m.setCursor(1)

	for range 5 {
		m = wheel(m, tea.MouseButtonWheelUp, 30)
	}
	require.Equal(t, 0, m.cursor(), "cursor reaches the first item")
	assert.True(t, m.wheel.dead, "reaching the list top empties the momentum queue")
}

// Navigating left (to the parent) empties the momentum queue so the tail does
// not scroll the list it returns to (user rule 2, the reporter's repro).
func TestNavigateLeftEmptiesWheelQueue(t *testing.T) {
	m := longListModel(30)
	m = wheel(m, tea.MouseButtonWheelDown, 30) // start a burst on the middle list
	require.False(t, m.wheel.dead)

	mdl, _ := m.navigateParent()
	assert.True(t, mdl.(Model).wheel.dead, "navigating left must empty the momentum queue")
}

// Switching (or reloading) a tab empties the momentum queue so a fling in one
// tab does not scroll the list of the tab it lands on (#524).
func TestTabSwitchEmptiesWheelQueue(t *testing.T) {
	m := longListModel(30)
	m.wheel.dead = false
	m.wheel.lastAt = time.Now()

	_ = m.loadTab(0)
	assert.True(t, m.wheel.dead, "switching tabs must empty the momentum queue")
}

// Drilling into / backing out of the Object Explorer empties the momentum
// queue, mirroring left/right navigation in the main explorer (#524).
func TestObjectExplorerNavEmptiesWheelQueue(t *testing.T) {
	m := baseExplorerModel()
	m.mode = modeObjectExplorer

	m.wheel.dead = false
	m.objectExplorerDrill()
	assert.True(t, m.wheel.dead, "drilling in must empty the momentum queue")

	m.wheel.dead = false
	m.objectExplorerBack()
	assert.True(t, m.wheel.dead, "backing out must empty the momentum queue")
}

// A decelerating trackpad-momentum tail has inter-tick gaps larger than a
// dense burst. After navigation voids the gesture, such a sparse tail tick must
// still be dropped, not misread as a fresh gesture that scrolls the list the
// nav landed on (#524 "jumps by 3 after navigating back").
func TestWheelMomentumTailAfterNavStaysDropped(t *testing.T) {
	m := longListModel(30)
	m.setCursor(10)
	m.wheel.dead = true          // navigation just voided the gesture
	m.wheel.target = "ex-cursor" // parent list is the same target kind
	m.wheel.lastAt = time.Now().Add(-200 * time.Millisecond)

	before := m.cursor()
	m = wheel(m, tea.MouseButtonWheelUp, 30)
	assert.Equal(t, before, m.cursor(),
		"a sparse momentum tail after navigation must not scroll the list")
}

// A deliberate new scroll after a real pause is a fresh burst and scrolls
// normally, even if the previous burst was marked dead at a boundary.
func TestNewWheelBurstScrollsAfterPause(t *testing.T) {
	m := longListModel(30)
	m.setCursor(5)
	m.wheel.dead = true
	m.wheel.target = "ex-cursor"
	m.wheel.lastAt = time.Now().Add(-time.Second) // long pause -> new burst

	m = wheel(m, tea.MouseButtonWheelDown, 30)
	assert.Greater(t, m.cursor(), 5, "a fresh burst after a pause scrolls normally")
}
