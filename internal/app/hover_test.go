package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

func hoverTestModel() Model {
	m := baseExplorerModel()
	items := make([]model.Item, 6)
	for i := range items {
		items[i] = model.Item{Name: "pod-" + string(rune('a'+i)), Kind: "Pod"}
	}
	m.middleItems = items
	m.setCursor(0)
	m.mouseAvailable, m.mouseCaptured = true, true
	_ = m.renderView() // fills the line map the hit test reads
	return m
}

// rowY finds a screen row whose hit test lands on item idx.
func rowY(t *testing.T, m Model, idx int) (x, y int) {
	t.Helper()
	leftEnd, _ := m.columnBoundaries()
	x = leftEnd + 4
	for y = 1; y < m.height; y++ {
		if got := m.hoverAt(x, y); got.surface == hoverExplorer && got.idx == idx {
			return x, y
		}
	}
	require.Failf(t, "row not found", "item %d", idx)
	return 0, 0
}

func hoverMotion(x, y int) tea.MouseMotionMsg {
	return tea.MouseMotionMsg{Button: tea.MouseNone, X: x, Y: y}
}

func TestHoverMotionSetsStateWithoutMovingCursor(t *testing.T) {
	m := hoverTestModel()
	x, y := rowY(t, m, 3)

	ret, _ := m.handleMouse(hoverMotion(x, y))
	got := ret.(Model)

	assert.Equal(t, hoverState{surface: hoverExplorer, idx: 3}, got.hover)
	assert.Equal(t, 0, got.cursor())
}

func TestHoverClearsWhenPointerLeavesList(t *testing.T) {
	m := hoverTestModel()
	x, y := rowY(t, m, 3)
	ret, _ := m.handleMouse(hoverMotion(x, y))
	m = ret.(Model)

	ret, _ = m.handleMouse(hoverMotion(0, 0))
	assert.Equal(t, hoverState{}, ret.(Model).hover)
}

func TestHoverClearedByKeyPress(t *testing.T) {
	m := hoverTestModel()
	m.hover = hoverState{surface: hoverExplorer, idx: 3}

	ret, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	assert.Equal(t, hoverState{}, ret.(Model).hover)
}

func TestHoverClearedByWheelAndClick(t *testing.T) {
	m := hoverTestModel()
	x, y := rowY(t, m, 3)
	m.hover = hoverState{surface: hoverExplorer, idx: 3}

	ret, _ := m.handleMouse(tea.MouseWheelMsg{Button: tea.MouseWheelDown, X: x, Y: y})
	assert.Equal(t, hoverState{}, ret.(Model).hover)

	ret, _ = m.handleMouse(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	got := ret.(Model)
	assert.Equal(t, hoverState{}, got.hover)
	assert.Equal(t, 3, got.cursor(), "click still selects the hovered row")
}

func TestHoverClearedByPaneDividerDrag(t *testing.T) {
	m := hoverTestModel()
	leftEnd, _ := m.columnBoundaries()
	m.hover = hoverState{surface: hoverExplorer, idx: 3}

	m = press(m, leftEnd-1, 5)
	require.Equal(t, dividerLeft, m.paneResize.drag)
	assert.Equal(t, hoverState{}, m.hover)
}

func TestHoverIgnoredWithoutMouseCapture(t *testing.T) {
	m := hoverTestModel()
	x, y := rowY(t, m, 3)
	m.mouseCaptured = false

	ret, _ := m.handleMouse(hoverMotion(x, y))
	assert.Equal(t, hoverState{}, ret.(Model).hover)

	m.hover = hoverState{surface: hoverExplorer, idx: 3}
	assert.Equal(t, -1, m.explorerHoverRow())
}

func TestHoverRowRendersDifferentlyOnlyInRawOutput(t *testing.T) {
	m := hoverTestModel()
	plain := m.renderView()

	m.hover = hoverState{surface: hoverExplorer, idx: 3}
	hovered := m.renderView()

	assert.Equal(t, stripANSI(plain), stripANSI(hovered), "hover only changes styling")
	assert.NotEqual(t, plain, hovered, "hovered row must be styled differently")

	// Capture off with mouse never available keeps the status bar identical.
	m.mouseAvailable, m.mouseCaptured = false, false
	assert.Equal(t, plain, m.renderView(), "no hover styling without mouse capture")
}

func TestHoverOnCursorRowKeepsSelectedStyle(t *testing.T) {
	m := hoverTestModel()
	plain := m.renderView()

	m.hover = hoverState{surface: hoverExplorer, idx: m.cursor()}
	assert.Equal(t, plain, m.renderView())
}

func TestMouseHoverFilter(t *testing.T) {
	m := hoverTestModel()
	x, y := rowY(t, m, 3)
	x2, y2 := rowY(t, m, 4)

	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(x, y)), "row change passes through")

	m.hover = hoverState{surface: hoverExplorer, idx: 3}
	assert.Nil(t, MouseHoverFilter(m, hoverMotion(x, y)), "same row is dropped")
	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(x2, y2)), "other row passes through")
	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(0, 0)), "leaving the list passes through")

	m.hover = hoverState{}
	assert.Nil(t, MouseHoverFilter(m, hoverMotion(0, 0)), "off-list motion with no hover is dropped")

	drag := tea.MouseMotionMsg{Button: tea.MouseLeft, X: x, Y: y}
	assert.NotNil(t, MouseHoverFilter(m, drag), "button motion passes through")

	m.paneResize.drag = dividerLeft
	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(0, 0)), "active drag passes through")

	assert.NotNil(t, MouseHoverFilter(m, tea.KeyPressMsg{Code: 'j'}), "other messages pass through")
}

func TestActionOverlayHover(t *testing.T) {
	m := actionOverlayModel(3)
	m.mouseAvailable, m.mouseCaptured = true, true
	plain := m.renderView()

	// box.x = 24, box.y = 11, item 1 sits at y = 16.
	ret, _ := m.handleMouse(hoverMotion(60, 16))
	got := ret.(Model)
	assert.Equal(t, hoverState{surface: hoverAction, idx: 1}, got.hover)
	assert.Equal(t, 0, got.overlayCursor)
	assert.NotEqual(t, plain, got.renderView())

	ret, _ = got.handleMouse(hoverMotion(2, 2))
	assert.Equal(t, hoverState{}, ret.(Model).hover)
}

func TestNamespaceOverlayHover(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	m.mouseAvailable, m.mouseCaptured = true, true
	_ = m.renderView()
	box, ok := m.centeredOverlayBox()
	require.True(t, ok)

	// itemsStartRow = 4 inside the content, which starts 2 rows below the box top.
	got := m.hoverAt(box.x+4, box.y+2+4+1)
	assert.Equal(t, hoverState{surface: hoverNamespace, idx: 1}, got)
}

func TestOverlayHoverAgreesWithClickOnScrolledNamespaceList(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	for i := range 40 {
		m.overlayItems = append(m.overlayItems, model.Item{Name: "ns-" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	m.overlayCursor = 30
	m.mouseAvailable, m.mouseCaptured = true, true
	_ = m.renderView()
	box, ok := m.centeredOverlayBox()
	require.True(t, ok)
	require.Positive(t, getOverlayGeom().scroll, "list must be scrolled for this test")

	hits := 0
	for y := box.y; y < box.y+box.h; y++ {
		click := m.overlayItemIndexAt(y - (box.y + 2))
		hover := m.hoverAt(box.x+4, y)
		if click < 0 {
			assert.Equal(t, hoverState{}, hover, "y=%d", y)
			continue
		}
		hits++
		assert.Equal(t, hoverState{surface: hoverNamespace, idx: click}, hover, "y=%d", y)
	}
	assert.Positive(t, hits)
}

func TestOverlayGeomWrittenOnlyByView(t *testing.T) {
	m := actionOverlayModel(3)
	m.mouseAvailable, m.mouseCaptured = true, true
	sentinel := overlayListGeom{kind: overlayAction, total: 99}
	setOverlayGeom(sentinel)

	_, _ = m.centeredOverlayBox()
	_, _ = m.handleMouse(tea.MouseClickMsg{Button: tea.MouseLeft, X: 60, Y: 16})
	assert.Equal(t, sentinel, getOverlayGeom())

	_ = m.renderView()
	assert.Equal(t, 3, getOverlayGeom().total)

	m.overlay = overlayNone
	_ = m.renderView()
	assert.Equal(t, overlayListGeom{}, getOverlayGeom(), "reset when no hoverable overlay is drawn")
}

func TestMouseHoverFilterOtherOverlays(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayConfirm
	m.mouseAvailable, m.mouseCaptured = true, true
	_ = m.renderView()

	assert.Nil(t, MouseHoverFilter(m, hoverMotion(60, 20)), "no hover to clear, motion dropped")

	m.hover = hoverState{surface: hoverExplorer, idx: 2}
	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(60, 20)), "one pass clears stale hover")
}

func TestMouseHoverFilterActionOverlay(t *testing.T) {
	m := actionOverlayModel(3)
	m.mouseAvailable, m.mouseCaptured = true, true
	_ = m.renderView()

	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(60, 16)))
	m.hover = hoverState{surface: hoverAction, idx: 1}
	assert.Nil(t, MouseHoverFilter(m, hoverMotion(60, 16)), "same row dropped")
	assert.NotNil(t, MouseHoverFilter(m, hoverMotion(60, 17)), "row change passes")
}

// screenPos finds where text first appears in the plain rendering.
func screenPos(t *testing.T, view, text string) (x, y int) {
	t.Helper()
	for y, line := range strings.Split(stripANSI(view), "\n") {
		if before, _, found := strings.Cut(line, text); found {
			return lipgloss.Width(before), y
		}
	}
	require.Failf(t, "text not on screen", "%q", text)
	return 0, 0
}

func TestNamespaceOverlayHoverFollowsRealPath(t *testing.T) {
	m := baseExplorerModel()
	m.overlay = overlayNamespace
	m.overlayItems = []model.Item{{Name: "alpha"}, {Name: "bravo"}, {Name: "charlie"}}
	m.mouseAvailable, m.mouseCaptured = true, true
	view := m.renderView()
	x, y := screenPos(t, view, "bravo")

	msg := hoverMotion(x, y)
	require.NotNil(t, MouseHoverFilter(m, msg), "filter must let a row change through")
	next, _ := m.Update(msg)
	hovered := next.(Model)
	require.Equal(t, hoverState{surface: hoverNamespace, idx: 1}, hovered.hover)

	lines := strings.Split(hovered.renderView(), "\n")
	plainLines := strings.Split(view, "\n")
	assert.NotEqual(t, plainLines[y], lines[y], "the bravo row must be restyled")
	assert.Equal(t, plainLines[y-1], lines[y-1])
	assert.Equal(t, stripANSI(plainLines[y+1]), stripANSI(lines[y+1]))
}

func TestNamespaceOverlayRowsBelowVisibleListResolveToNothing(t *testing.T) {
	m := baseExplorerModel()
	m.height = 24
	m.overlay = overlayNamespace
	for i := range 30 {
		m.overlayItems = append(m.overlayItems, model.Item{Name: "ns-" + string(rune('a'+i%26)) + string(rune('a'+i/26))})
	}
	m.mouseAvailable, m.mouseCaptured = true, true
	view := m.renderView()
	x, y := screenPos(t, view, "ns-a")

	visible := namespaceOverlayVisibleRows(m.namespaceOverlayH(), 30)
	require.Equal(t, 12, visible)
	last, below := y+visible-1, y+visible
	assert.Equal(t, hoverState{surface: hoverNamespace, idx: visible - 1}, m.hoverAt(x, last))
	assert.Equal(t, visible-1, m.overlayItemIndexAt(last-(boxTop(t, m)+2)))
	assert.Equal(t, hoverState{}, m.hoverAt(x, below))
	assert.Equal(t, -1, m.overlayItemIndexAt(below-(boxTop(t, m)+2)))
}

func boxTop(t *testing.T, m Model) int {
	t.Helper()
	box, ok := m.centeredOverlayBox()
	require.True(t, ok)
	return box.y
}
