package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

func TestSessionManagerKeyOpensOverlay(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	prev := ui.ActiveKeybindings.SessionManager
	ui.ActiveKeybindings.SessionManager = "C"
	t.Cleanup(func() { ui.ActiveKeybindings.SessionManager = prev })

	m := basePush80Model()
	ret, _, handled := m.handleExplorerUIKey(runeKey('C'))
	require.True(t, handled, "session-manager key handled")
	assert.Equal(t, overlaySessions, ret.(Model).overlay)
}

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

func TestSessionsOverlayShowsDefaultRow(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	rm := ret.(Model)
	// The built-in default workspace is always present, even with no saved
	// named sessions, so it stays reachable.
	rows := rm.sessionRows()
	require.NotEmpty(t, rows)
	assert.True(t, rows[0].isDefault)
	assert.Equal(t, "default", rows[0].label)
	assert.Contains(t, stripANSI(renderSessionsOverlay(rm)), "default")
}

// TestSessionsOverlayDefaultRowShowsLiveTabCount guards the reported bug: on the
// default session, opening new tabs (which do not persist session.yaml) must
// still show the live tab count, not the stale on-disk count.
func TestSessionsOverlayDefaultRowShowsLiveTabCount(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	// session.yaml on disk holds a single tab (the last persisted state).
	require.NoError(t, saveSession(SessionState{Tabs: []SessionTab{{Context: "a"}}}))

	m := basePush80Model() // activeSession == "" (default)
	m.tabs = []TabState{{}, {}, {}}
	m.activeTab = 0

	ret, _ := m.openSessionsOverlay()
	rm := ret.(Model)

	rows := rm.sessionRows()
	require.NotEmpty(t, rows)
	assert.True(t, rows[0].isDefault)
	assert.Equal(t, 3, rows[0].tabs, "default row shows live tab count, not the stale disk count")
	assert.Contains(t, stripANSI(renderSessionsOverlay(rm)), "3 tabs")
}

// TestSessionsOverlayActiveNamedRowShowsLiveTabCount extends the fix to a named
// session that is currently active: its row reflects the live tabs, while a
// non-active named session keeps its persisted count.
func TestSessionsOverlayActiveNamedRowShowsLiveTabCount(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{
		Name: "work", SavedAt: time.Now(),
		State: SessionState{Tabs: []SessionTab{{Context: "a"}, {Context: "b"}}},
	}))
	require.NoError(t, saveNamedSession(NamedSession{
		Name: "other", SavedAt: time.Now(),
		State: SessionState{Tabs: []SessionTab{{Context: "x"}}},
	}))

	m := basePush80Model()
	m.activeSession = "work"
	m.tabs = []TabState{{}, {}, {}, {}} // 4 live tabs
	m.activeTab = 0

	ret, _ := m.openSessionsOverlay()
	rm := ret.(Model)

	var work, other sessionRow
	for _, r := range rm.sessionRows() {
		switch r.name {
		case "work":
			work = r
		case "other":
			other = r
		}
	}
	assert.Equal(t, 4, work.tabs, "active named session shows live tab count")
	assert.Equal(t, 1, other.tabs, "inactive named session keeps its persisted count")
}

func TestSessionsOverlayEnterSwitches(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{
		Name: "prod", SavedAt: time.Now(),
		State: SessionState{Context: "ctx-x", Tabs: []SessionTab{{Context: "ctx-x"}}},
	}))
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	m = ret.(Model)
	m.overlayCursor = 1 // row 0 is the default; row 1 is "prod"

	ret, cmd := m.handleSessionsOverlayKey(specialKey(tea.KeyEnter))
	rm := ret.(Model)
	assert.Equal(t, overlayNone, rm.overlay, "overlay closes on switch")
	assert.Equal(t, "prod", rm.activeSession, "active session updated")
	require.NotNil(t, rm.pendingSession)
	assert.Equal(t, "ctx-x", rm.pendingSession.Context)
	assert.False(t, rm.sessionRestored, "restore must fire on next contexts load")
	assert.NotNil(t, cmd)
}

func TestSessionsOverlaySwitchToDefault(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{Name: "prod", SavedAt: time.Now(), State: SessionState{Context: "c", Tabs: []SessionTab{{Context: "c"}}}}))
	m := basePush80Model()
	m.activeSession = "prod"
	ret, _ := m.openSessionsOverlay()
	m = ret.(Model)
	m.overlayCursor = 0 // the default row

	ret, _ = m.handleSessionsOverlayKey(specialKey(tea.KeyEnter))
	rm := ret.(Model)
	assert.Equal(t, "", rm.activeSession, "switched to default")
	assert.Equal(t, "", loadActiveSessionName(), "default persisted (pointer removed)")
	assert.Equal(t, overlayNone, rm.overlay)
}

func TestSessionsOverlayDeleteRemovesRow(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{Name: "a", SavedAt: time.Now(), State: SessionState{Context: "c"}}))
	require.NoError(t, saveNamedSession(NamedSession{Name: "b", SavedAt: time.Now().Add(time.Second), State: SessionState{Context: "c"}}))
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	m = ret.(Model)
	require.Len(t, m.sessionsList, 2)
	m.overlayCursor = 1 // row 0 is default; row 1 is a named session

	ret, _ = m.handleSessionsOverlayKey(runeKey('d'))
	rm := ret.(Model)
	assert.Len(t, rm.sessionsList, 1, "list refreshed after delete")
}

func TestSessionsOverlayDeleteDefaultRejected(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{Name: "a", SavedAt: time.Now(), State: SessionState{Context: "c"}}))
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	m = ret.(Model)
	m.overlayCursor = 0 // the default row

	ret, _ = m.handleSessionsOverlayKey(runeKey('d'))
	rm := ret.(Model)
	assert.Len(t, rm.sessionsList, 1, "named sessions untouched")
	assert.True(t, rm.statusMessageErr, "deleting default is rejected")
}

func TestSessionsOverlaySaveAsPromptSavesAndActivates(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.nav.Context = "ctx-a"
	m.tabs[m.activeTab].nav.Context = "ctx-a"
	ret, _ := m.openSessionsOverlay()
	m = ret.(Model)

	// Press "s" to enter the name prompt, type "work", Enter to commit.
	ret, _ = m.handleSessionsOverlayKey(runeKey('s'))
	m = ret.(Model)
	require.True(t, m.sessionsSaveMode)
	for _, r := range "work" {
		ret, _ = m.handleSessionsOverlayKey(runeKey(r))
		m = ret.(Model)
	}
	ret, _ = m.handleSessionsOverlayKey(specialKey(tea.KeyEnter))
	rm := ret.(Model)

	assert.False(t, rm.sessionsSaveMode)
	assert.Equal(t, "work", rm.activeSession, "saved session becomes active")
	ns, err := loadNamedSession("work")
	require.NoError(t, err)
	assert.Equal(t, "ctx-a", ns.State.Context)
}

func TestSessionsOverlaySwitchLoadFailureAborts(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m := basePush80Model()
	m.activeSession = "current"
	// A named session that isn't on disk (e.g. deleted between list and switch)
	// must not abandon the current session or clobber its file.
	ret, cmd := m.switchToSession("ghost")
	rm := ret.(Model)
	assert.Equal(t, "current", rm.activeSession, "active session unchanged on load failure")
	assert.Nil(t, rm.pendingSession, "no restore armed")
	assert.True(t, rm.statusMessageErr)
	assert.NotNil(t, cmd)
}

func TestSessionsOverlaySwitchRejectedInUnion(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	require.NoError(t, saveNamedSession(NamedSession{Name: "p", SavedAt: time.Now(), State: SessionState{Context: "c"}}))
	m := basePush80Model()
	ret, _ := m.openSessionsOverlay()
	m = ret.(Model)
	m.unionMode = true
	ret, _ = m.handleSessionsOverlayKey(specialKey(tea.KeyEnter))
	rm := ret.(Model)
	assert.Nil(t, rm.pendingSession, "no switch while in union view")
	assert.True(t, rm.statusMessageErr)
}
