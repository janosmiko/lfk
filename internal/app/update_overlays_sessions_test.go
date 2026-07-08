package app

import (
	"testing"

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
