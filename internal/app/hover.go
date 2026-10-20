package app

import (
	"sync"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

type hoverSurface int

const (
	hoverNone hoverSurface = iota
	hoverExplorer
	hoverAction
	hoverNamespace
)

type hoverState struct {
	surface hoverSurface
	idx     int
}

// hoverFor returns the hovered item index when s is the active hover surface.
func (m Model) hoverFor(s hoverSurface) *int {
	if !m.mouseCaptured || m.hover.surface != s {
		return nil
	}
	return &m.hover.idx
}

// explorerHoverRow returns the middle-column row to highlight, or -1.
func (m Model) explorerHoverRow() int {
	if m.mode != modeExplorer || m.overlay != overlayNone {
		return -1
	}
	if idx := m.hoverFor(hoverExplorer); idx != nil {
		return *idx
	}
	return -1
}

// hoverAt hit-tests the pointer position against the hoverable lists.
func (m Model) hoverAt(x, y int) hoverState {
	if m.mode != modeExplorer {
		return hoverState{}
	}
	if m.overlay != overlayNone {
		return m.overlayHoverAt(x, y)
	}
	if y <= 0 || m.fullscreenDashboard {
		return hoverState{}
	}
	leftEnd, middleEnd := m.columnBoundaries()
	if x < leftEnd || x >= middleEnd {
		return hoverState{}
	}
	idx, isHeader, _ := m.resolveMiddleColumnClick(x, y, leftEnd)
	if isHeader || idx < 0 {
		return hoverState{}
	}
	return hoverState{surface: hoverExplorer, idx: idx}
}

// overlayListGeom is the list layout of the overlay the last View drew.
// Hover reads it instead of rendering, because View has a value receiver and
// cannot store it on the Model.
type overlayListGeom struct {
	kind      overlayKind
	box       overlayBoundingBox
	firstRowY int
	rowCount  int
	scroll    int
	total     int
}

var (
	overlayGeomMu     sync.RWMutex
	activeOverlayGeom overlayListGeom
)

func setOverlayGeom(g overlayListGeom) {
	overlayGeomMu.Lock()
	activeOverlayGeom = g
	overlayGeomMu.Unlock()
}

func getOverlayGeom() overlayListGeom {
	overlayGeomMu.RLock()
	defer overlayGeomMu.RUnlock()
	return activeOverlayGeom
}

// recordOverlayGeom stores the list layout. Call it only from the View path.
func (m Model) recordOverlayGeom(content string, ow, oh int) {
	g := overlayListGeom{kind: m.overlay, box: m.overlayBoxFor(content, ow, oh)}
	switch m.overlay {
	case overlayAction:
		g.total = len(m.overlayItems)
		g.rowCount, g.firstRowY = g.total, g.box.y+2+2
	case overlayNamespace:
		g.total = len(m.filteredOverlayItems())
		g.rowCount, g.firstRowY = min(15, g.total), g.box.y+2+4
		g.scroll = ui.OverlayNsScroll
	default:
		return
	}
	setOverlayGeom(g)
}

func (m Model) overlayHoverAt(x, y int) hoverState {
	g := getOverlayGeom()
	if g.kind != m.overlay || x < g.box.x || x >= g.box.x+g.box.w || y < g.box.y || y >= g.box.y+g.box.h {
		return hoverState{}
	}
	row := y - g.firstRowY
	idx := g.scroll + row
	if row < 0 || row >= g.rowCount || idx < 0 || idx >= g.total {
		return hoverState{}
	}
	if m.overlay == overlayAction {
		return hoverState{surface: hoverAction, idx: idx}
	}
	return hoverState{surface: hoverNamespace, idx: idx}
}

func (m Model) handleHover(mouse tea.Mouse) Model {
	if !m.mouseCaptured {
		m.hover = hoverState{}
		return m
	}
	m.hover = m.hoverAt(mouse.X, mouse.Y)
	return m
}

// MouseHoverFilter drops pointer motion that leaves hover unchanged, because
// Bubble Tea redraws after every message. Overlays hit-test from the layout
// the last View stored, so the filter never renders.
func MouseHoverFilter(model tea.Model, msg tea.Msg) tea.Msg {
	motion, ok := msg.(tea.MouseMotionMsg)
	if !ok || motion.Button != tea.MouseNone {
		return msg
	}
	var m Model
	switch v := model.(type) {
	case Model:
		m = v
	case *Model:
		m = *v
	default:
		return msg
	}
	if m.paneResize.drag != dividerNone {
		return msg
	}
	if m.hoverAt(motion.X, motion.Y) == m.hover {
		return nil
	}
	return msg
}
