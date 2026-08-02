package app

import (
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/janosmiko/lfk/internal/ui"
)

// handleNetpolSearchKey handles keyboard input while the network policy
// overlay's / search bar is active. Mirrors the describe-view search bar:
// printable keys live-update the highlight query, Enter commits and jumps
// to the first match, Esc cancels.
func (m Model) handleNetpolSearchKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.netpolSearchActive = false
		m.netpolSearchQuery = m.netpolSearchInput.Value
		// Anchor just above the viewport top so a match on the first
		// visible line counts as the first hit.
		m.netpolSearchPos = m.netpolScroll - 1
		return m.findNetpolMatch(true)
	case "esc":
		m.netpolSearchActive = false
		m.netpolSearchInput.Clear()
		m.netpolSearchQuery = ""
	case "backspace":
		if len(m.netpolSearchInput.Value) > 0 {
			m.netpolSearchInput.Backspace()
		}
		m.netpolSearchQuery = m.netpolSearchInput.Value
	case "ctrl+w":
		m.netpolSearchInput.DeleteWord()
		m.netpolSearchQuery = m.netpolSearchInput.Value
	case "ctrl+a":
		m.netpolSearchInput.Home()
	case "ctrl+e":
		m.netpolSearchInput.End()
	case "left":
		m.netpolSearchInput.Left()
	case "right":
		m.netpolSearchInput.Right()
	default:
		if msg.Text != "" {
			m.netpolSearchInput.Insert(msg.Text)
			// Live-update the highlight query so matches paint as the
			// user types instead of waiting for Enter to commit.
			m.netpolSearchQuery = m.netpolSearchInput.Value
		}
	}
	return m, nil
}

// netpolOverlayLines returns the full styled line list of whichever netpol
// view is loaded, at the same width the renderer draws with.
func (m Model) netpolOverlayLines() []string {
	_, innerW, _ := m.netpolOverlayDims()
	switch {
	case m.netpolsData != nil:
		return ui.NetworkPoliciesOverlayLines(convertNetpolsForResource(m.netpolsData), innerW)
	case m.netpolData != nil:
		return ui.NetworkPolicyOverlayLines(convertNetpolInfo(*m.netpolData), innerW)
	}
	return nil
}

// findNetpolMatch scrolls to the next/previous line matching the committed
// search query, wrapping around the content. The overlay has no cursor, so
// the last match index (netpolSearchPos) is the anchor — not the scroll
// offset, which clamps at maxScroll and would pin n/N at the bottom of the
// content. A match is brought to the top of the viewport when it can be.
func (m Model) findNetpolMatch(forward bool) (Model, tea.Cmd) {
	if m.netpolSearchQuery == "" {
		return m, nil
	}
	lines := m.netpolOverlayLines()
	total := len(lines)
	if total == 0 {
		return m, nil
	}
	start := m.netpolSearchPos
	for i := 1; i <= total; i++ {
		var idx int
		if forward {
			idx = (start + i) % total // start >= -1, i >= 1, so never negative
		} else {
			idx = ((start-i)%total + total) % total
		}
		// MatchLine applies the same substring/regex/fuzzy mode detection
		// the renderer's highlight uses, so n/N land on highlighted lines.
		if ui.MatchLine(ansi.Strip(lines[idx]), m.netpolSearchQuery) {
			m.netpolSearchPos = idx
			m.netpolScroll = min(idx, m.netpolMaxScroll())
			return m, nil
		}
	}
	m.setStatusMessage("Pattern not found: "+m.netpolSearchQuery, false)
	return m, scheduleStatusClear()
}

// handleNetpolWheel scrolls the network policy overlay on mouse wheel
// ticks. The 3-line step matches the explorer-mode wheel speed.
func (m Model) handleNetpolWheel(msg tea.MouseMsg) Model {
	mouse := msg.Mouse()
	const wheelStep = 3
	switch mouse.Button {
	case tea.MouseWheelDown:
		m.netpolScroll = min(m.netpolScroll+wheelStep, m.netpolMaxScroll())
	case tea.MouseWheelUp:
		m.netpolScroll = max(m.netpolScroll-wheelStep, 0)
	}
	return m
}

// closeNetpolOverlay resets all network policy overlay state and closes it.
func (m Model) closeNetpolOverlay() Model {
	m.netpolLineInput = ""
	m.overlay = overlayNone
	m.netpolData = nil
	m.netpolsData = nil
	m.netpolSearchActive = false
	m.netpolSearchInput.Clear()
	m.netpolSearchQuery = ""
	m.netpolSearchPos = 0
	return m
}
