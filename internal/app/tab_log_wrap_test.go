package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

// log_viewer.wrap seeds a new log viewer, but a tab switch must give the tab
// its own toggle back. Restoring from the global default instead of the
// snapshot dropped the user's `>` toggle on every switch.
func TestLoadTab_RestoresPerTabLogWrap(t *testing.T) {
	prev := ui.ConfigLogWrap
	t.Cleanup(func() { ui.ConfigLogWrap = prev })
	ui.ConfigLogWrap = false

	m := &Model{tabs: []TabState{{}, {}}}
	m.mode = modeLogs
	m.logView.wrap = true // the user pressed > on tab 0
	m.saveCurrentTab()

	m.loadTab(1)
	require.False(t, m.logView.wrap, "tab 1 never toggled wrap, so it stays unwrapped")

	m.loadTab(0)
	assert.True(t, m.logView.wrap, "switching back to tab 0 keeps the wrap toggle")
}
