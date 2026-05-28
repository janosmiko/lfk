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
// explorer navigation key, so a toast disappears as soon as the user moves on
// instead of waiting out its 5s timer. Covers cursor movement, page/preview
// scrolling, level jumps, owner/back jumps, search-match jumps and tab
// switching — anything that changes what the user is looking at.
//
// Called before key dispatch (handleExplorerKey), so a handler that sets a
// fresh message for the same key (e.g. JumpOwner reporting "no owner", or
// NewTab rejected in union mode) still overwrites the cleared state and wins.
// Returns m unchanged for any other key.
func (m Model) clearStatusOnNavigationKey(msg tea.KeyMsg) Model {
	kb := ui.ActiveKeybindings
	switch msg.String() {
	case kb.Down, "down", kb.Up, "up", kb.Left, "left", kb.Right, "right",
		kb.JumpTop, kb.JumpBottom, "end", "home",
		kb.PageDown, kb.PageUp, kb.PageForward, kb.PageBack, "pgdown", "pgup",
		kb.PreviewDown, kb.PreviewUp,
		kb.LevelCluster, kb.LevelTypes, kb.LevelResources,
		kb.JumpBack, kb.JumpOwner,
		kb.NextMatch, kb.PrevMatch,
		kb.NextTab, kb.PrevTab, kb.NewTab:
		m.clearTransientStatus()
	}
	return m
}
