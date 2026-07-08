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

func TestSaveCurrentSessionTargetsActiveSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.nav.Context = "ctx-a"
	m.tabs[m.activeTab].nav.Context = "ctx-a"

	// Default active session -> writes the legacy session.yaml.
	m.activeSession = ""
	m.saveCurrentSession()
	require.NotNil(t, loadSession(), "default active session writes session.yaml")
	if _, err := loadNamedSession("prod"); err == nil {
		t.Fatal("no named file should exist yet")
	}

	// Named active session -> writes sessions/prod.yaml, does NOT touch session.yaml.
	m.activeSession = "prod"
	m.saveCurrentSession()
	ns, err := loadNamedSession("prod")
	require.NoError(t, err)
	assert.Equal(t, "prod", ns.Name)
	assert.NotEmpty(t, ns.State.Tabs)
	assert.Equal(t, "ctx-a", ns.State.Context)
}

func TestLoadStartupSession(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Default: no cli, nothing persisted -> "" + whatever session.yaml holds (nil here).
	name, state := loadStartupSession("")
	assert.Equal(t, "", name)
	assert.Nil(t, state)

	// Existing named session loads its state.
	require.NoError(t, saveNamedSession(NamedSession{Name: "prod", State: SessionState{Context: "c", Tabs: []SessionTab{{Context: "c"}}}}))
	name, state = loadStartupSession("prod")
	assert.Equal(t, "prod", name)
	require.NotNil(t, state)
	assert.Equal(t, "c", state.Context)

	// Unknown session name -> create-on-use: active set, nil state (fresh workspace).
	name, state = loadStartupSession("brand-new")
	assert.Equal(t, "brand-new", name)
	assert.Nil(t, state)
	assert.Equal(t, "brand-new", loadActiveSessionName(), "cli session persisted as active")
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
