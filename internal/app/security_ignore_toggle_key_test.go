package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// TestSecurityIgnoreToggleKeyOnSecurityView drives the REAL explorer key
// dispatch (not the handler directly) to prove the configured
// SecurityIgnoreToggle key actually reaches the show-ignored toggle on a
// security view. This is the regression guard for the original bug: the
// default was "ctrl+i", which a terminal delivers as "tab" (byte 0x09), so
// the binding never fired — yet the handler-level unit tests passed because
// they bypassed the key string entirely.
func TestSecurityIgnoreToggleKeyOnSecurityView(t *testing.T) {
	m := Model{}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_heuristic__"}
	require.False(t, m.showSecurityIgnored)

	res, _, handled := m.handleExplorerActionKey(keyMsg(ui.ActiveKeybindings.SecurityIgnoreToggle))

	require.True(t, handled, "ignore toggle key must be handled on a security view")
	assert.True(t, res.(Model).showSecurityIgnored, "key press must flip showSecurityIgnored")
}

// TestSecurityIgnoreToggleNotHandledOffSecurityView verifies the toggle is
// scoped: off a security view the key must not flip the ignored state (it
// falls through to its global meaning, e.g. the label editor).
func TestSecurityIgnoreToggleNotHandledOffSecurityView(t *testing.T) {
	m := Model{}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "Pod"}

	_, _, handled := m.handleExplorerSecurityViewKeys(keyMsg(ui.ActiveKeybindings.SecurityIgnoreToggle))

	assert.False(t, handled, "ignore toggle must not fire off a security view")
}

// TestSecurityIgnoreToggleHelperOnSecurityView checks the gated helper in
// isolation: on a security view it consumes the key and flips the state.
func TestSecurityIgnoreToggleHelperOnSecurityView(t *testing.T) {
	m := Model{}
	m.nav.ResourceType = model.ResourceTypeEntry{Kind: "__security_falco__"}

	res, _, handled := m.handleExplorerSecurityViewKeys(keyMsg(ui.ActiveKeybindings.SecurityIgnoreToggle))

	require.True(t, handled)
	assert.True(t, res.(Model).showSecurityIgnored)
}
