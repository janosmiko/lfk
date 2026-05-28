package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/ui"
)

// clearTransientStatus immediately drops any active status/error message so it
// does not linger in the hint bar while the user navigates. The startup tip is
// dismissed separately in handleKey. No-op when nothing is showing.
func (m *Model) clearTransientStatus() {
	if m.statusMessage == "" {
		return
	}
	m.statusMessage = ""
	m.statusMessageErr = false
	m.statusMessageExp = time.Time{}
}

// clearStatusOnNavigationKey clears a lingering hint-bar message when msg is an
// explorer movement key (up/down/left/right plus the top/bottom jumps), so a
// toast disappears on the next navigation keystroke instead of waiting out its
// 5s timer. Returns m unchanged for any other key.
func (m Model) clearStatusOnNavigationKey(msg tea.KeyMsg) Model {
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.Down, "down", kb.Up, "up", kb.Left, "left", kb.Right, "right",
		kb.JumpTop, kb.JumpBottom, "end":
		m.clearTransientStatus()
	}
	return m
}
