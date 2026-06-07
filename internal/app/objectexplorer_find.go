package app

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// findResultLimit caps the recursive search to keep large objects responsive.
const findResultLimit = 1000

// openObjectExplorerFind opens the recursive key-search overlay, seeded with all
// node paths in the object. Mirrors the API Explorer's recursive browser (r).
func (m *Model) openObjectExplorerFind() {
	rt := &m.objectExplorerView
	rt.findFilter = ""
	rt.findFilterActive = false
	rt.findCursor = 0
	rt.findScroll = 0
	rt.findResults = model.AllObjectPaths(rt.root, findResultLimit)
	m.overlay = overlayObjectExplorerFind
}

// closeObjectExplorerFind dismisses the overlay and clears its results.
func (m *Model) closeObjectExplorerFind() {
	rt := &m.objectExplorerView
	rt.findResults = nil
	rt.findFilter = ""
	rt.findFilterActive = false
	rt.findCursor = 0
	rt.findScroll = 0
	m.overlay = overlayNone
}

// handleObjectExplorerFindKey handles input for the recursive-find overlay,
// delegating to the filter-input sub-handler when the filter is focused.
func (m Model) handleObjectExplorerFindKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.objectExplorerView.findFilterActive {
		return m.handleObjectExplorerFindFilterKey(msg)
	}
	rt := &m.objectExplorerView
	n := len(rt.findResults)
	_, _, visible := m.findOverlayDims()
	switch msg.String() {
	case "esc", "q":
		m.closeObjectExplorerFind()
	case "enter", "l", "right":
		m.jumpToFindResult()
	case "/":
		rt.findFilterActive = true
	case "j", "down":
		if rt.findCursor < n-1 {
			rt.findCursor++
		}
	case "k", "up":
		if rt.findCursor > 0 {
			rt.findCursor--
		}
	case "g", "home":
		rt.findCursor = 0
	case "G", "end":
		rt.findCursor = max(n-1, 0)
	case "ctrl+d", "pgdown", "shift+down":
		rt.findCursor = min(rt.findCursor+visible/2, max(n-1, 0))
	case "ctrl+u", "pgup", "shift+up":
		rt.findCursor = max(rt.findCursor-visible/2, 0)
	}
	m.clampFindScroll()
	return m, nil
}

// handleObjectExplorerFindFilterKey handles typing in the overlay's filter input.
func (m Model) handleObjectExplorerFindFilterKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	rt := &m.objectExplorerView
	switch msg.String() {
	case "esc":
		// Leave the filter input and clear it (matches the API Explorer);
		// the overlay stays open showing all results.
		rt.findFilterActive = false
		rt.findFilter = ""
		m.recomputeFind()
	case "enter":
		rt.findFilterActive = false
		rt.findCursor = 0
		rt.findScroll = 0
	case "backspace":
		if rt.findFilter != "" {
			rt.findFilter = rt.findFilter[:len(rt.findFilter)-1]
		}
		m.recomputeFind()
	default:
		if msg.Type == tea.KeyRunes {
			rt.findFilter += string(msg.Runes)
			m.recomputeFind()
		}
	}
	m.clampFindScroll()
	return m, nil
}

// recomputeFind refreshes the result list for the current filter.
func (m *Model) recomputeFind() {
	rt := &m.objectExplorerView
	if rt.findFilter == "" {
		rt.findResults = model.AllObjectPaths(rt.root, findResultLimit)
	} else {
		rt.findResults = model.FindObjectPaths(rt.root, rt.findFilter, findResultLimit)
	}
	rt.findCursor = max(0, min(rt.findCursor, len(rt.findResults)-1))
	rt.findScroll = 0
}

// jumpToFindResult navigates the tree to the selected match (path set to the
// match's parent, cursor onto the matched key) and closes the overlay.
func (m *Model) jumpToFindResult() {
	rt := &m.objectExplorerView
	if rt.findCursor < 0 || rt.findCursor >= len(rt.findResults) {
		m.closeObjectExplorerFind()
		return
	}
	segs := rt.findResults[rt.findCursor].Segs
	m.navigateObjectExplorerToPath(segs)
	m.closeObjectExplorerFind()
}

// navigateObjectExplorerToPath sets the tree's path to segs' parent and the
// cursor onto the final segment's key. A no-op for an empty path.
func (m *Model) navigateObjectExplorerToPath(segs []string) {
	rt := &m.objectExplorerView
	if len(segs) == 0 {
		return
	}
	parent := segs[:len(segs)-1]
	key := segs[len(segs)-1]
	rt.path = append([]string{}, parent...)
	rt.level = model.ObjectFieldsAt(rt.root, rt.path)
	rt.resetLevelView()
	for i, f := range rt.level {
		if f.Key == key {
			rt.cursor = i
			break
		}
	}
	m.clampObjectExplorerScroll()
}

// findOverlayDims returns the recursive-find overlay box width, height, and the
// number of result rows visible inside it. Shared by the renderer and the
// scroll-clamp so the scrollbar and cursor stay in sync.
func (m Model) findOverlayDims() (w, h, maxVisible int) {
	w = min(m.width-6, max(m.width*70/100, 64))
	h = min(m.height-4, max(m.height*70/100, 12))
	maxVisible = max(h-8, 1) // title + subtitle + filter(2) + footer(2) + borders
	return w, h, maxVisible
}

// clampFindScroll keeps the find cursor within the visible results window.
func (m *Model) clampFindScroll() {
	rt := &m.objectExplorerView
	_, _, visible := m.findOverlayDims()
	if rt.findCursor < rt.findScroll {
		rt.findScroll = rt.findCursor
	}
	if rt.findCursor >= rt.findScroll+visible {
		rt.findScroll = rt.findCursor - visible + 1
	}
	if rt.findScroll < 0 {
		rt.findScroll = 0
	}
}

// renderOverlayObjectExplorerFind builds the centered recursive-find overlay
// using the shared OverlayList renderer (scrollbar, filter prompt, footer).
func (m Model) renderOverlayObjectExplorerFind() (string, int, int) {
	rt := m.objectExplorerView
	w, h, maxVisible := m.findOverlayDims()

	items := make([]ui.OverlayListItem, len(rt.findResults))
	for i, r := range rt.findResults {
		items[i] = ui.OverlayListItem{Name: formatObjectPath(r.Segs), Description: r.Preview}
	}
	cfg := ui.OverlayListConfig{
		Title:           "Find in " + rt.title,
		Subtitle:        fmt.Sprintf("%d %s", len(items), pluralize(len(items), "match", "matches")),
		Cursor:          rt.findCursor,
		Filterable:      true,
		Filter:          rt.findFilter,
		FilterActive:    rt.findFilterActive,
		ShowDescription: true,
		Scroll:          rt.findScroll,
		MaxVisible:      maxVisible,
		EmptyMessage:    "No matching keys",
		Height:          h - 2,
	}
	return ui.RenderOverlayList(items, cfg, w-4), w, h
}

// pluralize returns singular when n == 1, else plural.
func pluralize(n int, singular, plural string) string {
	if n == 1 {
		return singular
	}
	return plural
}
