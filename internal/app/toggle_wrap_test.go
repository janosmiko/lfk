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

// TestViewerToggleBindings_Defaults pins the default keys for the viewer display
// toggles so a careless rebinding of the struct is caught.
func TestViewerToggleBindings_Defaults(t *testing.T) {
	kb := ui.DefaultKeybindings()
	assert.Equal(t, "#", kb.ToggleLineNumbers, "toggle_line_numbers")
	assert.Equal(t, "z", kb.ToggleFold, "toggle_fold")
	assert.Equal(t, "Z", kb.ToggleFoldAll, "toggle_fold_all")
	assert.Equal(t, "f", kb.ToggleFollow, "toggle_follow")
	assert.Equal(t, "s", kb.ToggleTimestamps, "toggle_timestamps")
	assert.Equal(t, "p", kb.TogglePrefixes, "toggle_prefixes")
	assert.Equal(t, "u", kb.ToggleUnified, "toggle_unified")
}

func diffToggleModel() Model {
	m := baseModelHandlers2()
	m.mode = modeDiff
	m.diffView.left = "a"
	m.diffView.right = "b"
	return m
}

// TestDiffToggles_ViaDefaultKeys verifies the diff display toggles fire on their
// configurable bindings' default keys.
func TestDiffToggles_ViaDefaultKeys(t *testing.T) {
	m := diffToggleModel()
	m.diffView.unified = false
	r1, _ := m.handleDiffKey(keyMsg("u"))
	assert.True(t, r1.(Model).diffView.unified, "u toggles unified")

	m2 := diffToggleModel()
	m2.diffView.lineNumbers = false
	r2, _ := m2.handleDiffKey(keyMsg("#"))
	assert.True(t, r2.(Model).diffView.lineNumbers, "# toggles line numbers")
}

// TestFullscreen_ShiftFEverywhere verifies the maximize/minimize action is the
// shared Fullscreen binding (Shift+F) in the event viewer, and that the old
// lowercase `f` no longer toggles it there.
func TestFullscreen_ShiftFEverywhere(t *testing.T) {
	assert.Equal(t, "F", ui.DefaultKeybindings().Fullscreen, "fullscreen default is Shift+F")

	// F maximizes from the minimized overlay.
	max, _ := newEventModel(10).handleEventTimelineOverlayKey(runeKey('F'))
	assert.Equal(t, modeEventViewer, max.(Model).mode, "F maximizes the event timeline")

	// f no longer maximizes (freed).
	noop, _ := newEventModel(10).handleEventTimelineOverlayKey(runeKey('f'))
	assert.NotEqual(t, modeEventViewer, noop.(Model).mode, "f no longer maximizes")
}

// TestViewerSearch_HonorsRebind verifies the YAML viewer now respects a rebound
// `search` keybinding (category B: viewers used to hardcode "/").
func TestViewerSearch_HonorsRebind(t *testing.T) {
	orig := ui.ActiveKeybindings
	t.Cleanup(func() { ui.ActiveKeybindings = orig })
	ui.ActiveKeybindings.Search = "ctrl+y"

	bound, _ := yamlWrapModel().handleYAMLKey(keyMsg("ctrl+y"))
	assert.True(t, bound.(Model).yamlView.searchMode, "rebound search key should enter search")

	old, _ := yamlWrapModel().handleYAMLKey(keyMsg("/"))
	assert.False(t, old.(Model).yamlView.searchMode, "/ should not search after rebind")
}
