package app

import (
	"maps"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/model"
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
	// With perLevel on, levels holds each level's saved split and the fields
	// above are only the working pair of a drag. Never write through levels:
	// Model copies share the map.
	perLevel bool
	levels   map[model.Level][2]int
}

// at returns the state with the split of level loaded, so apply and the drag
// start see that level's widths. Outside per-level mode, or mid-drag, it is s.
func (s paneResizeState) at(level model.Level) paneResizeState {
	if !s.perLevel || s.drag != dividerNone {
		return s
	}
	p := s.levels[level]
	s.leftPermille, s.rightPermille = p[0], p[1]
	return s
}

// withLevel returns a copy of the levels map with level set, or removed when
// both values are 0.
func (s paneResizeState) withLevel(level model.Level, left, right int) map[model.Level][2]int {
	out := make(map[model.Level][2]int, len(s.levels)+1)
	maps.Copy(out, s.levels)
	if left == 0 && right == 0 {
		delete(out, level)
	} else {
		out[level] = [2]int{left, right}
	}
	return out
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
		m, cmd := m.finishPaneDrag()
		return m, cmd, true
	}
	return m, nil, false
}

// finishPaneDrag ends a drag and saves the split when it changed.
func (m Model) finishPaneDrag() (Model, tea.Cmd) {
	s := m.paneResize
	s.drag = dividerNone
	level := m.nav.Level
	changed := s.leftPermille != s.startLeft || s.rightPermille != s.startRight
	var cmd tea.Cmd
	switch {
	case !changed:
		s.leftPermille, s.rightPermille = s.priorLeft, s.priorRight
	case s.perLevel:
		s.levels = s.withLevel(level, s.leftPermille, s.rightPermille)
		cmd = m.reportPrefSaveErr(persistPaneWidthsPrefs(level, s.leftPermille, s.rightPermille))
	default:
		cmd = m.reportPrefSaveErr(persistPanePrefs(s.leftPermille, s.rightPermille))
	}
	m.paneResize = paneResizeState{
		leftPermille: s.leftPermille, rightPermille: s.rightPermille,
		lastPressAt: s.lastPressAt, lastPressOn: s.lastPressOn,
		perLevel: s.perLevel, levels: s.levels,
	}
	if changed {
		m.paneResize.lastPressOn = dividerNone
	}
	return m, cmd
}

func (m Model) startPaneDrag(mouse tea.Mouse) (Model, tea.Cmd, bool) {
	if mouse.Y <= 0 || m.fullscreenMiddle || m.fullscreenDashboard {
		return m, nil, false
	}
	m.paneResize = m.paneResize.at(m.nav.Level)
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
// With per-level widths, only the current level's pair changes.
func (m Model) resetPaneWidth(d paneDivider) (Model, tea.Cmd) {
	left, right := m.paneResize.leftPermille, m.paneResize.rightPermille
	if d == dividerLeft {
		left = 0
	} else {
		right = 0
	}
	had := left != m.paneResize.leftPermille || right != m.paneResize.rightPermille
	s := paneResizeState{leftPermille: left, rightPermille: right, perLevel: m.paneResize.perLevel, levels: m.paneResize.levels}
	save := func() error { return persistPanePrefs(left, right) }
	if s.perLevel {
		level := m.nav.Level
		s.levels = s.withLevel(level, left, right)
		save = func() error { return persistPaneWidthsPrefs(level, left, right) }
	}
	m.paneResize = s
	if !had {
		return m, nil
	}
	cmd := m.reportPrefSaveErr(save())
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
