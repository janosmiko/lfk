package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func press(m Model, x, y int) Model {
	ret, _ := m.handleMouse(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: y})
	return ret.(Model)
}

func motion(m Model, x, y int) Model {
	ret, _ := m.handleMouse(tea.MouseMotionMsg{Button: tea.MouseLeft, X: x, Y: y})
	return ret.(Model)
}

func release(m Model, x, y int) Model {
	ret, _ := m.handleMouse(tea.MouseReleaseMsg{Button: tea.MouseLeft, X: x, Y: y})
	return ret.(Model)
}

func TestPaneResizeDefaultsUnchanged(t *testing.T) {
	for _, w := range []int{40, 80, 100, 120, 200, 333} {
		usable := w - 6
		m := baseExplorerModel()
		m.width = w
		l, mid, r := m.explorerColumnWidths()
		wantL := max(10, usable*12/100)
		wantM := max(10, usable*51/100)
		assert.Equal(t, []int{wantL, wantM, max(10, usable-wantL-wantM)}, []int{l, mid, r}, "width %d", w)

		m.hideLeftPane = true
		l, mid, r = m.explorerColumnWidths()
		wantM = max(10, usable*58/100)
		assert.Equal(t, []int{0, wantM, max(10, usable-wantM)}, []int{l, mid, r}, "hidden width %d", w)
	}
}

func TestWidthToPermilleRoundTrip(t *testing.T) {
	for usable := 20; usable <= 400; usable += 7 {
		for w := minPaneWidth; w <= usable-2*minPaneWidth; w++ {
			assert.Equal(t, w, usable*widthToPermille(w, usable)/1000, "w=%d usable=%d", w, usable)
		}
	}
}

func TestPaneResizeDragLeftDividerKeepsRight(t *testing.T) {
	m := baseExplorerModel()
	_, _, rightBefore := m.explorerColumnWidths()
	leftEnd, _ := m.columnBoundaries()

	m = press(m, leftEnd-1, 5)
	require.Equal(t, dividerLeft, m.paneResize.drag)
	m = motion(m, 30, 5)

	l, mid, r := m.explorerColumnWidths()
	assert.Equal(t, 29, l, "grabbed border follows the pointer")
	assert.Equal(t, rightBefore, r)
	assert.Equal(t, 114-l-r, mid)
	nl, _ := m.columnBoundaries()
	assert.Equal(t, 30, nl-1)
}

func TestPaneResizeDragLeftDividerMiddleBorderCell(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd, 5)
	m = motion(m, 31, 12)
	nl, _ := m.columnBoundaries()
	assert.Equal(t, 31, nl)
}

func TestPaneResizeDragRightDividerKeepsLeft(t *testing.T) {
	m := baseExplorerModel()
	leftBefore, _, _ := m.explorerColumnWidths()
	_, middleEnd := m.columnBoundaries()

	m = press(m, middleEnd-1, 5)
	require.Equal(t, dividerRight, m.paneResize.drag)
	m = motion(m, 90, 5)

	l, mid, r := m.explorerColumnWidths()
	assert.Equal(t, leftBefore, l)
	_, me := m.columnBoundaries()
	assert.Equal(t, 90, me-1, "grabbed border follows the pointer")
	assert.Equal(t, 114-l-r, mid)
}

func TestPaneResizeClampsMinWidth(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, middleEnd := m.columnBoundaries()

	left := press(m, leftEnd-1, 5)
	left = motion(left, 0, 5)
	l, _, _ := left.explorerColumnWidths()
	assert.Equal(t, minPaneWidth, l)
	left = motion(left, 119, 5)
	l, mid, r := left.explorerColumnWidths()
	assert.Equal(t, minPaneWidth, mid)
	assert.GreaterOrEqual(t, r, minPaneWidth)
	assert.Equal(t, 114, l+mid+r)

	right := press(m, middleEnd-1, 5)
	right = motion(right, 119, 5)
	_, _, r = right.explorerColumnWidths()
	assert.Equal(t, minPaneWidth, r)
	right = motion(right, 0, 5)
	l, mid, r = right.explorerColumnWidths()
	assert.Equal(t, minPaneWidth, mid)
	assert.GreaterOrEqual(t, l, minPaneWidth)
	assert.Equal(t, 114, l+mid+r)
}

func TestPaneResizeHideLeftPaneOnlyRightDivider(t *testing.T) {
	m := baseExplorerModel()
	m.hideLeftPane = true
	// x=0..1 would be the left divider if the pane existed.
	m = press(m, 0, 5)
	assert.Equal(t, dividerNone, m.paneResize.drag)

	_, middleEnd := m.columnBoundaries()
	m = press(m, middleEnd, 5)
	require.Equal(t, dividerRight, m.paneResize.drag)
	m = motion(m, 80, 5)
	l, _, _ := m.explorerColumnWidths()
	assert.Zero(t, l)
	_, me := m.columnBoundaries()
	assert.Equal(t, 80, me)
}

func TestPaneResizeFullscreenIgnoresPress(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m.fullscreenMiddle = true
	m = press(m, leftEnd-1, 5)
	assert.Equal(t, dividerNone, m.paneResize.drag)
	m.fullscreenMiddle = false
	m.fullscreenDashboard = true
	m = press(m, leftEnd-1, 5)
	assert.Equal(t, dividerNone, m.paneResize.drag)
}

func TestPaneResizeFullscreenIgnoresOverrides(t *testing.T) {
	m := baseExplorerModel()
	m.paneResize.leftPermille, m.paneResize.rightPermille = 300, 300
	m.fullscreenMiddle = true
	l, mid, r := m.explorerColumnWidths()
	assert.Equal(t, []int{0, 118, 0}, []int{l, mid, r})
}

func TestPaneResizePressDoesNotNavigate(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	assert.Equal(t, model.LevelResources, m.nav.Level)
}

func TestPaneResizeTitleBarRowIsNotDivider(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 0)
	assert.Equal(t, dividerNone, m.paneResize.drag)
}

func TestPaneResizeReleaseEndsDrag(t *testing.T) {
	isolateViewerPrefs(t)
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = motion(m, 30, 5)
	m = release(m, 30, 9)
	assert.Equal(t, dividerNone, m.paneResize.drag)

	before := m.paneResize
	m = motion(m, 50, 5)
	assert.Equal(t, before, m.paneResize)
}

func TestPaneResizeMotionWithoutDragIsNoop(t *testing.T) {
	m := baseExplorerModel()
	m = motion(m, 50, 5)
	assert.Equal(t, paneResizeState{}, m.paneResize)
}

func TestPaneResizeNormalPressClearsStaleDrag(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = press(m, 60, 5)
	assert.Equal(t, dividerNone, m.paneResize.drag)
}

func TestPaneResizeKeepsProportionsOnTerminalResize(t *testing.T) {
	isolateViewerPrefs(t)
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = motion(m, 40, 5)
	m = release(m, 40, 5)

	l1, _, _ := m.explorerColumnWidths()
	m.width = 240
	l2, _, _ := m.explorerColumnWidths()
	assert.InDelta(t, float64(l1)/114, float64(l2)/234, 0.02)
}

func TestPaneResizePreviewLogInnerWidthFollowsOverride(t *testing.T) {
	m := baseExplorerModel()
	_, _, r0 := m.explorerColumnWidths()
	assert.Equal(t, r0-2, m.previewLogInnerWidth())

	_, middleEnd := m.columnBoundaries()
	m = press(m, middleEnd-1, 5)
	m = motion(m, 70, 5)
	_, _, r1 := m.explorerColumnWidths()
	assert.NotEqual(t, r0, r1)
	assert.Equal(t, r1-2, m.previewLogInnerWidth())
}

func TestPaneResizeMiddleRowClickStillSelects(t *testing.T) {
	prev := ui.ActiveMiddleLineMap
	ui.ActiveMiddleLineMap = []int{0, 1, 2}
	t.Cleanup(func() { ui.ActiveMiddleLineMap = prev })

	m := baseExplorerModel()
	m = press(m, 40, 4)
	assert.Equal(t, 1, m.cursor())
	assert.Equal(t, dividerNone, m.paneResize.drag)
}

func widthsSum(m Model) (int, int, int, int) {
	l, mid, r := m.explorerColumnWidths()
	return l, mid, r, l + mid + r
}

func TestPaneResizeShrinkAfterExtremeLeftDrag(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = motion(m, 119, 5)
	m = release(m, 119, 5)
	m.width = 80
	l, mid, r, sum := widthsSum(m)
	assert.Equal(t, 74, sum)
	assert.GreaterOrEqual(t, l, minPaneWidth)
	assert.GreaterOrEqual(t, mid, minPaneWidth)
	assert.GreaterOrEqual(t, r, minPaneWidth)
}

func TestPaneResizeShrinkAfterExtremeRightDrag(t *testing.T) {
	m := baseExplorerModel()
	_, middleEnd := m.columnBoundaries()
	m = press(m, middleEnd-1, 5)
	m = motion(m, 0, 5)
	m = release(m, 0, 5)
	m.width = 80
	l, mid, r, sum := widthsSum(m)
	assert.Equal(t, 74, sum)
	assert.GreaterOrEqual(t, l, minPaneWidth)
	assert.GreaterOrEqual(t, mid, minPaneWidth)
	assert.GreaterOrEqual(t, r, minPaneWidth)
}

func TestPaneResizeSumEqualsUsable(t *testing.T) {
	for _, hide := range []bool{false, true} {
		for usable := 30; usable <= 300; usable++ {
			for _, lp := range []int{0, 1, 100, 500, 999, 1000} {
				for _, rp := range []int{0, 1, 100, 500, 999, 1000} {
					if lp == 0 && rp == 0 {
						continue
					}
					m := baseExplorerModel()
					m.width = usable + 6
					m.hideLeftPane = hide
					m.paneResize.leftPermille, m.paneResize.rightPermille = lp, rp
					l, mid, r, sum := widthsSum(m)
					if sum != usable || min(mid, r) < minPaneWidth || (!hide && l < minPaneWidth) {
						t.Fatalf("usable=%d hide=%v lp=%d rp=%d got %d/%d/%d", usable, hide, lp, rp, l, mid, r)
					}
				}
			}
		}
	}
}

func TestPaneResizeStartAtTinyWidth(t *testing.T) {
	m := baseExplorerModel()
	m.width = 24
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m = motion(m, 5, 5)
	l, mid, r := m.explorerColumnWidths()
	assert.GreaterOrEqual(t, min(l, mid, r), minPaneWidth)
}

func TestPaneResizeFullscreenDuringDragEndsIt(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m.fullscreenMiddle = true
	m = motion(m, 30, 5)
	assert.Equal(t, dividerNone, m.paneResize.drag)
}

func TestPaneResizeReleaseAfterOverlayOpensClearsDrag(t *testing.T) {
	m := baseExplorerModel()
	leftEnd, _ := m.columnBoundaries()
	m = press(m, leftEnd-1, 5)
	m.overlay = overlayAction
	m = release(m, 30, 5)
	assert.Equal(t, dividerNone, m.paneResize.drag)
}
