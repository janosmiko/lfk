package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

type paneDivider int

const (
	dividerNone paneDivider = iota
	dividerLeft
	dividerRight
)

const (
	minPaneWidth      = 10
	doubleClickWindow = 400 * time.Millisecond
)

// paneClock is swapped in tests so double-click timing needs no sleeps.
var paneClock = time.Now

// paneResizeState holds the dragged column split. A permille of 0 means the
// default width. Permille keeps the proportions across terminal resizes.
type paneResizeState struct {
	drag          paneDivider
	grabOffset    int // 0 or 1: which of the two border cells was grabbed
	leftPermille  int
	rightPermille int
	// Permille values at press time, to tell whether the drag changed anything.
	startLeft, startRight int
	// Overrides before the press, restored when the drag changed nothing.
	priorLeft, priorRight int
	// Last press on a divider, for double-click detection.
	lastPressAt time.Time
	lastPressOn paneDivider
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
		dragged := m.paneResize.drag
		m.paneResize.drag = dividerNone
		s := m.paneResize
		changed := s.leftPermille != s.startLeft || s.rightPermille != s.startRight
		// Drop the pin on the other pane, or a double-click on this border could
		// not reset the layout to the default.
		if dragged == dividerLeft {
			s.rightPermille = s.priorRight
		} else {
			s.leftPermille = s.priorLeft
		}
		var cmd tea.Cmd
		if changed {
			cmd = m.reportPrefSaveErr(persistPanePrefs(s.leftPermille, s.rightPermille))
		} else {
			s.leftPermille, s.rightPermille = s.priorLeft, s.priorRight
		}
		m.paneResize = paneResizeState{
			leftPermille: s.leftPermille, rightPermille: s.rightPermille,
			lastPressAt: s.lastPressAt, lastPressOn: s.lastPressOn,
		}
		if changed {
			m.paneResize.lastPressOn = dividerNone
		}
		return m, cmd, true
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
		m.paneResize.lastPressOn = dividerNone
		return m, nil, false
	}

	now := paneClock()
	if m.paneResize.lastPressOn == m.paneResize.drag && now.Sub(m.paneResize.lastPressAt) <= doubleClickWindow {
		m, cmd := m.resetPaneWidth(m.paneResize.drag)
		return m, cmd, true
	}
	m.paneResize.lastPressAt, m.paneResize.lastPressOn = now, m.paneResize.drag
	m.paneResize.priorLeft, m.paneResize.priorRight = m.paneResize.leftPermille, m.paneResize.rightPermille
	// Pin the other pane so only the dragged border moves.
	m.paneResize.rightPermille = widthToPermille(rightW, usable)
	if leftW > 0 {
		m.paneResize.leftPermille = widthToPermille(leftW, usable)
	}
	m.paneResize.startLeft, m.paneResize.startRight = m.paneResize.leftPermille, m.paneResize.rightPermille
	return m, nil, true
}

// resetPaneWidth restores the default width of the pane on the outer side of
// divider d and forgets its saved width. The other divider keeps its split.
func (m Model) resetPaneWidth(d paneDivider) (Model, tea.Cmd) {
	left, right := m.paneResize.leftPermille, m.paneResize.rightPermille
	if d == dividerLeft {
		left = 0
	} else {
		right = 0
	}
	had := left != m.paneResize.leftPermille || right != m.paneResize.rightPermille
	m.paneResize = paneResizeState{leftPermille: left, rightPermille: right}
	if !had {
		return m, nil
	}
	cmd := m.reportPrefSaveErr(persistPanePrefs(left, right))
	return m.recomposeThemedContent(), cmd
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
