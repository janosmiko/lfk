package app

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActiveSessionRoundTrip(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Absent file -> default ("").
	assert.Equal(t, "", loadActiveSessionName())

	require.NoError(t, saveActiveSessionName("prod-debug"))
	assert.Equal(t, "prod-debug", loadActiveSessionName())

	// Empty name removes the pointer -> back to default.
	require.NoError(t, saveActiveSessionName(""))
	assert.Equal(t, "", loadActiveSessionName())
}

func TestResolveStartupSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// No CLI value, nothing persisted -> default.
	assert.Equal(t, "", resolveStartupSession(""))

	// CLI value wins and is persisted so a later no-arg start reopens it.
	assert.Equal(t, "staging", resolveStartupSession("  staging  "))
	assert.Equal(t, "staging", loadActiveSessionName())
	assert.Equal(t, "staging", resolveStartupSession(""))

	// A different CLI value overrides and re-persists.
	assert.Equal(t, "prod", resolveStartupSession("prod"))
	assert.Equal(t, "prod", loadActiveSessionName())
}
