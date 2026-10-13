package app

import (
	tea "charm.land/bubbletea/v2"
)

type paneDivider int

const (
	dividerNone paneDivider = iota
	dividerLeft
	dividerRight
)

const minPaneWidth = 10

// paneResizeState holds the dragged column split. A permille of 0 means the
// default width. Permille keeps the proportions across terminal resizes.
type paneResizeState struct {
	drag          paneDivider
	grabOffset    int // 0 or 1: which of the two border cells was grabbed
	leftPermille  int
	rightPermille int
}

func (s paneResizeState) active() bool { return s.leftPermille != 0 || s.rightPermille != 0 }

// apply swaps in the dragged left and right widths and makes the three sum to
// usable. When they do not fit, left shrinks first, then right, down to the minimum.
func (s paneResizeState) apply(usable, leftW, middleW, rightW int) (int, int, int) {
	if !s.active() {
		return leftW, middleW, rightW
	}
	if s.leftPermille != 0 && leftW > 0 {
		leftW = max(minPaneWidth, usable*s.leftPermille/1000)
	}
	if s.rightPermille != 0 {
		rightW = max(minPaneWidth, usable*s.rightPermille/1000)
	}
	excess := leftW + rightW + minPaneWidth - usable
	if cut := min(max(excess, 0), max(leftW-minPaneWidth, 0)); cut > 0 {
		leftW -= cut
		excess -= cut
	}
	if cut := min(max(excess, 0), max(rightW-minPaneWidth, 0)); cut > 0 {
		rightW -= cut
	}
	return leftW, max(minPaneWidth, usable-leftW-rightW), rightW
}

// widthToPermille rounds up so usable*p/1000 floors back to exactly w.
func widthToPermille(w, usable int) int {
	if usable <= 0 {
		return 0
	}
	return (w*1000 + usable - 1) / usable
}

// handlePaneDrag starts, updates, and ends a border drag. It reports whether
// it consumed the event.
func (m Model) handlePaneDrag(msg tea.MouseMsg, mouse tea.Mouse) (Model, tea.Cmd, bool) {
	switch msg.(type) {
	case tea.MouseClickMsg:
		if mouse.Button != tea.MouseLeft {
			return m, nil, false
		}
		// A new press always clears a drag whose release was lost.
		m.paneResize.drag = dividerNone
		if m.mode != modeExplorer || m.overlay != overlayNone {
			return m, nil, false
		}
		return m.startPaneDrag(mouse)
	case tea.MouseMotionMsg:
		if m.paneResize.drag == dividerNone {
			return m, nil, false
		}
		if m.mode != modeExplorer || m.overlay != overlayNone {
			m.paneResize.drag = dividerNone
			return m, nil, false
		}
		return m.dragPaneTo(mouse.X), nil, true
	case tea.MouseReleaseMsg:
		if m.paneResize.drag == dividerNone {
			return m, nil, false
		}
		m.paneResize.drag = dividerNone
		return m, nil, true
	}
	return m, nil, false
}

func (m Model) startPaneDrag(mouse tea.Mouse) (Model, tea.Cmd, bool) {
	if mouse.Y <= 0 || m.fullscreenMiddle || m.fullscreenDashboard {
		return m, nil, false
	}
	leftW, _, rightW := m.explorerColumnWidths()
	leftEnd, middleEnd := m.columnBoundaries()
	usable := m.width - 6

	switch {
	case leftW > 0 && (mouse.X == leftEnd-1 || mouse.X == leftEnd):
		m.paneResize.drag = dividerLeft
		m.paneResize.grabOffset = mouse.X - (leftEnd - 1)
	case mouse.X == middleEnd-1 || mouse.X == middleEnd:
		m.paneResize.drag = dividerRight
		m.paneResize.grabOffset = mouse.X - (middleEnd - 1)
	default:
		return m, nil, false
	}
	// Pin the other pane so only the dragged border moves.
	m.paneResize.rightPermille = widthToPermille(rightW, usable)
	if leftW > 0 {
		m.paneResize.leftPermille = widthToPermille(leftW, usable)
	}
	return m, nil, true
}

func (m Model) dragPaneTo(x int) Model {
	if m.fullscreenMiddle || m.fullscreenDashboard {
		m.paneResize.drag = dividerNone
		return m
	}
	leftW, middleW, rightW := m.explorerColumnWidths()
	usable := m.width - 6
	border := x - m.paneResize.grabOffset

	switch m.paneResize.drag {
	case dividerLeft:
		w := min(max(border-1, minPaneWidth), usable-rightW-minPaneWidth)
		m.paneResize.leftPermille = widthToPermille(max(w, minPaneWidth), usable)
	case dividerRight:
		leftEnd, _ := m.columnBoundaries()
		w := min(max(usable-leftW-(border-1-leftEnd), minPaneWidth), usable-leftW-minPaneWidth)
		m.paneResize.rightPermille = widthToPermille(max(w, minPaneWidth), usable)
	}
	l2, mid2, r2 := m.explorerColumnWidths()
	if l2 != leftW || mid2 != middleW || r2 != rightW {
		m = m.recomposeThemedContent()
	}
	return m
}
