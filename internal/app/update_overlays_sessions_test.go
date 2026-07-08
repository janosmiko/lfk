package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionCommandSaveWritesFile(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.tabs[m.activeTab].nav.Context = "ctx-a"

	_, _ = m.executeSessionCommand([]string{"save", "prod-debug"})

	got, err := loadNamedSession("prod-debug")
	require.NoError(t, err)
	assert.Equal(t, "prod-debug", got.Name)
	assert.NotEmpty(t, got.State.Tabs)
}

func TestSessionCommandSaveRequiresName(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	ret, _ := m.executeSessionCommand([]string{"save"})
	rm := ret.(Model)
	assert.Contains(t, rm.statusMessage, "Usage: :session save")
	assert.True(t, rm.statusMessageErr)
}

func TestSessionCommandDelete(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{Name: "gone", State: SessionState{Context: "c"}}))
	m := basePush80Model()

	ret, _ := m.executeSessionCommand([]string{"delete", "gone"})
	rm := ret.(Model)
	assert.Contains(t, rm.statusMessage, "Deleted session: gone")
	_, err := loadNamedSession("gone")
	assert.Error(t, err)
}

func TestSessionCommandSaveRejectedInUnionMode(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.unionMode = true
	ret, _ := m.executeSessionCommand([]string{"save", "x"})
	rm := ret.(Model)
	assert.True(t, rm.statusMessageErr)
	assert.Contains(t, rm.statusMessage, "union")
}

func TestSessionsOverlayRendersList(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{
		Name: "prod-debug", SavedAt: time.Now().Add(-2 * time.Hour),
		State: SessionState{Tabs: []SessionTab{{Context: "a"}, {Context: "b"}, {Context: "c"}}},
	}))
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	rm := ret.(Model)
	assert.Equal(t, overlaySessions, rm.overlay)
	require.Len(t, rm.sessionsList, 1)

	out := stripANSI(renderSessionsOverlay(rm))
	assert.Contains(t, out, "prod-debug")
	assert.Contains(t, out, "3 tabs")
}

func TestSessionsOverlayEmptyState(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	rm := ret.(Model)
	out := stripANSI(renderSessionsOverlay(rm))
	assert.Contains(t, out, "No saved sessions")
}
