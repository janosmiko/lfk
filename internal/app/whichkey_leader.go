package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/ui"
)

// whichKeyState holds the which-key popup state, shared by the g prefix and the
// leader panel. seq is a generation counter: a delayed reveal carries the seq it
// was scheduled with, so a tick from a superseded arming is dropped instead of
// flashing a stale panel. scroll is the panel's row offset, clamped at render
// time against the content the current terminal size can show.
type whichKeyState struct {
	armed  bool // leader pressed, waiting for the next key
	shown  bool // delay elapsed, panel drawn
	seq    int
	scroll int
}

// whichKeyLeaderTickMsg reveals the leader panel once the delay elapses.
type whichKeyLeaderTickMsg struct{ seq int }

// armWhichKeyLeader is called on every leader keypress. The panel scrolls
// rather than pages, so a repeat press while already armed has nothing to
// advance to and toggles the panel shut instead. With the default zero delay
// the panel shows at once; which_key_leader_delay_ms instead schedules a
// reveal tick.
func (m Model) armWhichKeyLeader() (Model, tea.Cmd) {
	if !ui.ConfigWhichKeyEnabled {
		m.whichKey = whichKeyState{}
		return m, nil
	}
	if m.whichKey.armed {
		return m.disarmWhichKeyLeader(), nil
	}
	m.whichKey.seq++
	m.whichKey.armed = true
	m.whichKey.scroll = 0
	if ui.ConfigWhichKeyLeaderDelayMs <= 0 {
		m.whichKey.shown = true
		return m, nil
	}
	m.whichKey.shown = false
	seq := m.whichKey.seq
	d := time.Duration(ui.ConfigWhichKeyLeaderDelayMs) * time.Millisecond
	return m, tea.Tick(d, func(time.Time) tea.Msg { return whichKeyLeaderTickMsg{seq: seq} })
}

// disarmWhichKeyLeader hides the panel and resets its scroll. Bumping seq
// invalidates any tick still in flight.
func (m Model) disarmWhichKeyLeader() Model {
	m.whichKey.armed = false
	m.whichKey.shown = false
	m.whichKey.scroll = 0
	m.whichKey.seq++
	return m
}

// whichKeyLeaderGroups turns the currently-available actions into render groups,
// in the declared group order. Groups with no available action are omitted, and
// each group's entries are ordered by sortWhichKeyCells rather than by their
// position in the catalog.
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
			sortWhichKeyCells(cells)
			out = append(out, whichKeyGroupCells{Title: string(g), Cells: cells})
		}
	}
	return out
}

// scrollWhichKey moves the panel viewport by half a page, the same half-page
// step ctrl+d/ctrl+u take everywhere else in the app. A no-op when everything
// already fits.
func (m Model) scrollWhichKey(groups []whichKeyGroupCells, down bool) Model {
	lay, ok := m.whichKeyLayoutFor(groups)
	if !ok || lay.maxScroll == 0 {
		return m
	}
	step := max(lay.viewRows/2, 1)
	if down {
		m.whichKey.scroll = min(m.whichKey.scroll+step, lay.maxScroll)
	} else {
		m.whichKey.scroll = max(m.whichKey.scroll-step, 0)
	}
	return m
}

// handleWhichKeyScrollKey consumes the half-page scroll keys for a visible
// panel. Callers MUST gate this on whichKey.shown so ctrl+d/ctrl+u keep their
// normal explorer action whenever no panel is on screen.
func (m Model) handleWhichKeyScrollKey(msg tea.KeyPressMsg, groups []whichKeyGroupCells) (Model, bool) {
	kb := ui.ActiveKeybindings
	switch key := msg.String(); {
	case kb.PageDown != "" && key == kb.PageDown:
		return m.scrollWhichKey(groups, true), true
	case kb.PageUp != "" && key == kb.PageUp:
		return m.scrollWhichKey(groups, false), true
	}
	return m, false
}

// whichKeyLeaderIntercept applies the leader panel's "any key but the leader
// closes it" rule, shared by handleKey and handleExplorerKey. It reports
// whether the key was consumed outright: the scroll keys are (they move the
// visible panel), esc is while the panel is actually SHOWN, and every other key
// still falls through to whatever it would normally do.
//
// esc is consumed only while shown so closing it is a visible effect. The
// default delay is 0, but a user who configures which_key_leader_delay_ms has a
// window where the panel is not drawn yet, and swallowing esc there steals it
// from handleExplorerEsc (clear selection / search / filter / preset, exit
// fullscreen, close tab) with no on-screen feedback at all.
func (m Model) whichKeyLeaderIntercept(msg tea.KeyPressMsg) (Model, bool) {
	if !m.whichKey.armed || msg.String() == ui.ActiveKeybindings.WhichKeyLeader {
		return m, false
	}
	if m.whichKey.shown {
		if out, scrolled := m.handleWhichKeyScrollKey(msg, m.whichKeyLeaderGroups()); scrolled {
			return out, true
		}
	}
	shown := m.whichKey.shown
	m = m.disarmWhichKeyLeader()
	return m, shown && msg.String() == "esc"
}

// renderWhichKeyLeader draws the leader panel when it is armed and visible.
// The panel has no in-box footer — the scroll and close hints live in the
// bottom hint bar (statusBar), matching every other overlay in this app.
func (m Model) renderWhichKeyLeader(background string) string {
	if !m.whichKey.armed || !m.whichKey.shown || !ui.ConfigWhichKeyEnabled {
		return background
	}
	return m.renderWhichKeyPanel(background, m.whichKeyLeaderGroups(), m.whichKey.scroll)
}
