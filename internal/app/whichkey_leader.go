package app

import (
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyState holds the which-key popup state, shared by the g prefix and the
// space leader. seq is a generation counter: a delayed reveal carries the seq it
// was scheduled with, so a tick from a superseded arming is dropped instead of
// flashing a stale panel. page is the leader's raw page counter — it only ever
// increases while armed and is read back modulo the current page count, so it
// never needs to know that count itself to wrap correctly.
type whichKeyState struct {
	armed bool // space pressed, waiting for the next key
	shown bool // delay elapsed, panel drawn
	seq   int
	page  int
}

// whichKeyLeaderTickMsg reveals the space-leader panel once the delay elapses.
type whichKeyLeaderTickMsg struct{ seq int }

// armWhichKeyLeader is called every time space fires the toggle-select handler,
// whether that is a fresh arming or a repeat press while already armed. A fresh
// arming resets to page 1; a repeat press while still armed advances the page
// instead, so "space space space" pages through the panel while each press
// keeps toggling selection. With no delay the panel shows at once; otherwise a
// tick reveals it, so a fast space-j-space multi-select never flashes the
// panel.
func (m Model) armWhichKeyLeader() (Model, tea.Cmd) {
	if !ui.ConfigWhichKeyEnabled {
		m.whichKey = whichKeyState{}
		return m, nil
	}
	if m.whichKey.armed {
		m.whichKey.page++
	} else {
		m.whichKey.page = 0
	}
	m.whichKey.seq++
	m.whichKey.armed = true
	if ui.ConfigWhichKeyLeaderDelayMs <= 0 {
		m.whichKey.shown = true
		return m, nil
	}
	m.whichKey.shown = false
	seq := m.whichKey.seq
	d := time.Duration(ui.ConfigWhichKeyLeaderDelayMs) * time.Millisecond
	return m, tea.Tick(d, func(time.Time) tea.Msg { return whichKeyLeaderTickMsg{seq: seq} })
}

// disarmWhichKeyLeader hides the panel and resets its page. Bumping seq
// invalidates any tick still in flight.
func (m Model) disarmWhichKeyLeader() Model {
	m.whichKey.armed = false
	m.whichKey.shown = false
	m.whichKey.page = 0
	m.whichKey.seq++
	return m
}

// whichKeyLeaderGroups turns the currently-available actions into render groups,
// in the declared group order. Groups with no available action are omitted.
func (m Model) whichKeyLeaderGroups() []whichKeyGroupCells {
	acts := m.availableWhichKeyActions()
	kb := ui.ActiveKeybindings
	byGroup := map[whichKeyGroup][]whichKeyCell{}
	for _, a := range acts {
		byGroup[a.Group] = append(byGroup[a.Group], whichKeyCell{key: a.Key(kb), desc: a.Label})
	}
	out := make([]whichKeyGroupCells, 0, len(byGroup))
	for _, g := range whichKeyGroupOrder() {
		if cells := byGroup[g]; len(cells) > 0 {
			out = append(out, whichKeyGroupCells{Title: string(g), Cells: cells})
		}
	}
	return out
}

// whichKeyLeaderPage returns the groups for the space leader's current page,
// the page index, and the page count. Pagination reuses the renderer's own
// geometry (whichKeyPanelGeometry) and per-group layout (renderWhichKeyGroups,
// via paginateWhichKeyGroups) rather than re-deriving row-fitting arithmetic,
// so the count this reports can never disagree with what renderWhichKeyLeader
// actually draws. One row is always reserved for the page-indicator footer,
// matching renderWhichKeyPanel's own reservation when footer != "".
func (m Model) whichKeyLeaderPage() ([]whichKeyGroupCells, int, int) {
	groups := m.whichKeyLeaderGroups()
	targetInner, maxInner, maxBodyRows, ok := m.whichKeyPanelGeometry()
	if !ok {
		return nil, 0, 0
	}
	maxBodyRows--
	if maxBodyRows < 1 {
		return nil, 0, 0
	}
	keyStyle, descStyle, titleStyle := whichKeyStyles()
	rendered := renderWhichKeyGroups(groups, targetInner, maxInner, keyStyle, descStyle, titleStyle)
	pages := paginateWhichKeyGroups(rendered, maxBodyRows)
	if len(pages) == 0 {
		return nil, 0, 0
	}
	idx := m.whichKey.page % len(pages)
	return pages[idx], idx, len(pages)
}

// renderWhichKeyLeader draws the space-leader panel when it is armed and
// visible. The page indicator is the panel's only in-box footer — the
// "space: more / esc: close" hotkey hint lives in the bottom hint bar
// (statusBar), matching every other overlay in this app.
func (m Model) renderWhichKeyLeader(background string) string {
	if !m.whichKey.armed || !m.whichKey.shown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	groups, idx, count := m.whichKeyLeaderPage()
	if len(groups) == 0 {
		return background
	}
	footer := fmt.Sprintf("(%d/%d)", idx+1, count)
	return m.renderWhichKeyPanel(background, groups, footer)
}
