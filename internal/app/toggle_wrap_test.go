package app

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/janosmiko/lfk/internal/ui"
)

// TestToggleWrap_DefaultBinding verifies the unified line-wrap keybinding
// defaults to ">".
func TestToggleWrap_DefaultBinding(t *testing.T) {
	assert.Equal(t, ">", ui.DefaultKeybindings().ToggleWrap)
}

// yamlWrapModel builds a minimal YAML-viewer model for wrap tests.
func yamlWrapModel() Model {
	return Model{
		width: 80, height: 30, mode: modeYAML,
		yamlView: yamlViewState{
			content:   "apiVersion: v1\nkind: Pod",
			collapsed: map[string]bool{},
			cursor:    0,
		},
		tabs: []TabState{{}},
	}
}

// TestToggleWrap_DefaultKeyWrapsYAML verifies ">" toggles wrap in the YAML
// viewer via the configurable binding.
func TestToggleWrap_DefaultKeyWrapsYAML(t *testing.T) {
	m := yamlWrapModel()
	ret, _ := m.handleYAMLKey(keyMsg(">"))
	assert.True(t, ret.(Model).yamlView.wrap)
}

// TestToggleWrap_Rebind verifies rebinding toggle_wrap changes the active wrap
// key: the new key wraps and the old default no longer does.
func TestToggleWrap_Rebind(t *testing.T) {
	orig := ui.ActiveKeybindings
	t.Cleanup(func() { ui.ActiveKeybindings = orig })

	ui.ActiveKeybindings.ToggleWrap = "ctrl+y"

	// New key wraps.
	bound, _ := yamlWrapModel().handleYAMLKey(keyMsg("ctrl+y"))
	assert.True(t, bound.(Model).yamlView.wrap, "rebound key should toggle wrap")

	// Old default no longer wraps (">" now falls through to other handling).
	old, _ := yamlWrapModel().handleYAMLKey(keyMsg(">"))
	assert.False(t, old.(Model).yamlView.wrap, "> should not wrap after rebind")
}

// TestToggleWrap_CtrlWNoLongerWrapsYAML guards the unification: ctrl+w used to
// toggle wrap in the YAML viewer but is now freed.
func TestToggleWrap_CtrlWNoLongerWrapsYAML(t *testing.T) {
	m := yamlWrapModel()
	ret, _ := m.handleYAMLKey(keyMsg("ctrl+w"))
	assert.False(t, ret.(Model).yamlView.wrap, "ctrl+w should no longer toggle wrap")
}
