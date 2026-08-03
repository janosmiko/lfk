package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// mouseToggleKey is the default MouseToggle binding, Ctrl+Option+Y, which
// Bubble Tea reports as "ctrl+alt+y" (modifiers print in a fixed order).
var mouseToggleKey = tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl | tea.ModAlt}

func TestMouseToggleSuspendsAndResumesCapture(t *testing.T) {
	m := baseExplorerModel()
	m.mouseAvailable = true
	m.mouseCaptured = true

	// First press suspends capture and emits a command (DisableMouse batch).
	ret, cmd := m.handleKey(mouseToggleKey)
	r := ret.(Model)
	assert.False(t, r.mouseCaptured, "first toggle suspends mouse capture")
	assert.NotNil(t, cmd, "suspending capture emits a command")

	// Second press resumes capture.
	ret2, cmd2 := r.handleKey(mouseToggleKey)
	r2 := ret2.(Model)
	assert.True(t, r2.mouseCaptured, "second toggle resumes mouse capture")
	assert.NotNil(t, cmd2, "resuming capture emits a command")
}

func TestMouseToggleNoOpWhenUnavailable(t *testing.T) {
	m := baseExplorerModel()
	m.mouseAvailable = false
	m.mouseCaptured = false

	ret, _ := m.handleKey(mouseToggleKey)
	r := ret.(Model)
	assert.False(t, r.mouseCaptured, "toggle is a no-op when mouse was never available")
	assert.Contains(t, r.statusMessage, "disabled", "explains why nothing happened")
}

// While a viewer search input is focused the toggle key must fall through
// to that input rather than flipping mouse capture.
func TestMouseToggleIgnoredDuringViewerSearch(t *testing.T) {
	m := baseExplorerModel()
	m.mode = modeYAML
	m.mouseAvailable = true
	m.mouseCaptured = true
	m.yamlView.searchMode = true

	_, _, handled := m.handleMouseToggleKey(mouseToggleKey)
	assert.False(t, handled, "toggle must not fire while a viewer search input is focused")
}

// An empty MouseToggle binding disables the feature entirely.
func TestMouseToggleEmptyBindingDoesNotFire(t *testing.T) {
	prev := ui.ActiveKeybindings.MouseToggle
	ui.ActiveKeybindings.MouseToggle = ""
	t.Cleanup(func() { ui.ActiveKeybindings.MouseToggle = prev })

	m := baseExplorerModel()
	m.mouseAvailable = true
	m.mouseCaptured = true

	_, _, handled := m.handleMouseToggleKey(mouseToggleKey)
	assert.False(t, handled, "no binding means the key is never intercepted")
}
