package app

import (
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.helpFilterActive {
		return m.handleHelpFilterInput(msg)
	}
	if m.helpSearchActive {
		return m.handleHelpSearchInput(msg)
	}

	switch msg.String() {
	case "esc":
		// Esc cascades: clear search highlights → clear filter → close.
		// Lets the user back out of search/filter state without losing
		// their place on the help screen (close-on-first-Esc would
		// require navigating back from scratch).
		switch {
		case m.helpSearchQuery != "":
			m.helpSearchQuery = ""
			m.helpMatchLines = nil
			m.helpMatchIdx = 0
			return m, nil
		case m.helpFilter.Value != "":
			m.helpFilter.Clear()
			m.helpScroll = 0
			return m, nil
		default:
			m.mode = m.helpPreviousMode
			return m, nil
		}
	case "q", "?", "f1":
		m.mode = m.helpPreviousMode
		return m, nil
	case "j", "down":
		m.helpScroll++
		m.clampHelpScroll()
		return m, nil
	case "k", "up":
		if m.helpScroll > 0 {
			m.helpScroll--
		}
		return m, nil
	case "g":
		if m.pendingG {
			m.pendingG = false
			m.helpScroll = 0
			return m, nil
		}
		m.pendingG = true
		return m, nil
	case "G":
		// Use a sentinel and clamp — clampHelpScroll knows the actual
		// max from the current visible help lines, so the model lands
		// exactly at the bottom instead of parking far past it.
		m.helpScroll = 9999
		m.clampHelpScroll()
		return m, nil
	case "ctrl+d":
		m.helpScroll += m.height / 2
		m.clampHelpScroll()
		return m, nil
	case "ctrl+u":
		m.helpScroll -= m.height / 2
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "ctrl+f", "pgdown":
		m.helpScroll += m.height
		m.clampHelpScroll()
		return m, nil
	case "ctrl+b", "pgup":
		m.helpScroll -= m.height
		if m.helpScroll < 0 {
			m.helpScroll = 0
		}
		return m, nil
	case "home":
		m.helpScroll = 0
		m.pendingG = false
		return m, nil
	case "end":
		m.helpScroll = 9999
		m.clampHelpScroll()
		return m, nil
	case "/":
		// Open search input. Search highlights matches inline without
		// removing non-matching lines (different from f-filter).
		m.helpSearchActive = true
		m.helpSearchInput.SetValue(m.helpSearchQuery)
		m.helpSearchInput.Focus()
		return m, textinput.Blink
	case "f":
		// Open filter input. Filter narrows the visible help to lines
		// matching the query.
		m.helpFilterActive = true
		m.helpSearchInput.SetValue(m.helpFilter.Value)
		m.helpSearchInput.Focus()
		return m, textinput.Blink
	case "n":
		// Navigate to next search match (after / + Enter).
		m.helpJumpToMatch(1)
		return m, nil
	case "N":
		m.helpJumpToMatch(-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	}
	return m, nil
}

// handleHelpSearchInput runs while the user is typing in the / search
// input. Updates helpSearchQuery on every keystroke (so highlights
// follow the typed text), supports ctrl+n / ctrl+p to jump between
// matches in real time, Enter to apply, Esc to cancel.
func (m Model) handleHelpSearchInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.helpSearchActive = false
		m.helpSearchQuery = ""
		m.helpMatchLines = nil
		m.helpMatchIdx = 0
		m.helpSearchInput.Blur()
		return m, nil
	case "enter":
		m.helpSearchActive = false
		m.helpSearchInput.Blur()
		// Keep helpSearchQuery so highlights persist and n/N navigate
		// after the input closes.
		return m, nil
	case "ctrl+n":
		m.helpJumpToMatch(1)
		return m, nil
	case "ctrl+p":
		m.helpJumpToMatch(-1)
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		var cmd tea.Cmd
		m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
		m.helpSearchQuery = m.helpSearchInput.Value()
		m.helpRecomputeMatches()
		// Jump to the first match so the user sees the highlight without
		// having to manually scroll. Nothing happens if there are no
		// matches.
		if len(m.helpMatchLines) > 0 {
			m.helpMatchIdx = 0
			m.helpScrollToMatch()
		}
		return m, cmd
	}
}

// handleHelpFilterInput runs while the user is typing in the f filter
// input. Filter narrows visible lines as the user types; Enter applies
// (closes input), Esc clears.
func (m Model) handleHelpFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.helpFilterActive = false
		m.helpFilter.Clear()
		m.helpSearchInput.Blur()
		m.helpScroll = 0
		return m, nil
	case "enter":
		m.helpFilterActive = false
		m.helpSearchInput.Blur()
		return m, nil
	case "ctrl+c":
		return m.closeTabOrQuit()
	default:
		var cmd tea.Cmd
		m.helpSearchInput, cmd = m.helpSearchInput.Update(msg)
		m.helpFilter.Value = m.helpSearchInput.Value()
		m.helpScroll = 0
		// A filter change can shift line indices, invalidating any prior
		// search match cursor — recompute against the new visible set.
		m.helpRecomputeMatches()
		return m, cmd
	}
}

// helpRecomputeMatches walks the current help lines (after filter is
// applied) and records the indices of lines containing the search
// query. Resets helpMatchIdx when the match set changes.
func (m *Model) helpRecomputeMatches() {
	m.helpMatchLines = nil
	if m.helpSearchQuery == "" {
		m.helpMatchIdx = 0
		return
	}
	lines := ui.BuildHelpLines(m.helpFilter.Value, m.helpContextMode)
	for i, line := range lines {
		if ui.MatchLine(line, m.helpSearchQuery) {
			m.helpMatchLines = append(m.helpMatchLines, i)
		}
	}
	if m.helpMatchIdx >= len(m.helpMatchLines) {
		m.helpMatchIdx = 0
	}
}

// helpJumpToMatch advances the match cursor by delta and scrolls the
// viewport so the new match line is visible. No-op when there are no
// matches.
func (m *Model) helpJumpToMatch(delta int) {
	if len(m.helpMatchLines) == 0 {
		return
	}
	m.helpMatchIdx = (m.helpMatchIdx + delta) % len(m.helpMatchLines)
	if m.helpMatchIdx < 0 {
		m.helpMatchIdx += len(m.helpMatchLines)
	}
	m.helpScrollToMatch()
}

// helpScrollToMatch positions helpScroll so the current match line sits
// roughly in the middle of the visible viewport — gives the user
// surrounding context instead of pinning the match to the top edge.
func (m *Model) helpScrollToMatch() {
	if len(m.helpMatchLines) == 0 {
		return
	}
	target := m.helpMatchLines[m.helpMatchIdx]
	visible := m.helpVisibleLines()
	m.helpScroll = max(target-visible/2, 0)
	m.clampHelpScroll()
}

// helpCurrentMatchLine returns the line index in the current
// (post-filter) help line list of the match the n/N cursor sits on,
// or -1 when there is no active search match. The renderer uses this
// to render the current match with the distinct
// SelectedSearchHighlightStyle so the user can tell which match the
// next n/N press will move from.
func (m *Model) helpCurrentMatchLine() int {
	if len(m.helpMatchLines) == 0 || m.helpMatchIdx < 0 || m.helpMatchIdx >= len(m.helpMatchLines) {
		return -1
	}
	return m.helpMatchLines[m.helpMatchIdx]
}

// helpVisibleLines returns the number of help-content rows that fit
// inside the overlay box. Defers to ui.HelpVisibleLines so the app's
// clamp/scroll-to-match math matches the renderer's display clamp
// exactly — when the formulas drift, "↓ more below" never disappears
// because the clamp stops short of the renderer's actual bottom.
//
// Pass the same screen height view.go passes to RenderHelpScreen
// (m.height - 1), not m.height — the bottom row is reserved for the
// status bar.
func (m *Model) helpVisibleLines() int {
	return ui.HelpVisibleLines(m.height - 1)
}

// clampHelpScroll bounds m.helpScroll to [0, max] where max is the
// largest scroll offset that still keeps the last help line in view.
// Without this clamp, G/end set helpScroll to 9999 and ctrl+d past
// the end keeps incrementing — both park the model way past the valid
// range, so subsequent ctrl+u presses spend dozens of keystrokes
// undoing phantom scroll before the viewport visibly moves.
func (m *Model) clampHelpScroll() {
	totalLines := len(ui.BuildHelpLines(m.helpFilter.Value, m.helpContextMode))
	maxScroll := max(totalLines-m.helpVisibleLines(), 0)
	if m.helpScroll > maxScroll {
		m.helpScroll = maxScroll
	}
	if m.helpScroll < 0 {
		m.helpScroll = 0
	}
}
