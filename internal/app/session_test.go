package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- sessionFilePath ---

func TestSessionFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		path := sessionFilePath()
		assert.Equal(t, "/custom/state/lfk/session.yaml", path)
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		path := sessionFilePath()
		assert.Contains(t, path, ".local/state/lfk/session.yaml")
	})
}

// --- migrateStateFile ---

func TestMigrateStateFile(t *testing.T) {
	t.Run("no legacy file returns nil", func(t *testing.T) {
		tmpDir := t.TempDir()
		newPath := filepath.Join(tmpDir, "new", "file.yaml")
		data := migrateStateFile("nonexistent.yaml", newPath)
		assert.Nil(t, data)
	})

	t.Run("migrates legacy file", func(t *testing.T) {
		// Create a fake home dir with legacy config.
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		legacyDir := filepath.Join(tmpDir, ".config", "lfk")
		require.NoError(t, os.MkdirAll(legacyDir, 0o755))

		legacyContent := []byte("test: data\n")
		require.NoError(t, os.WriteFile(filepath.Join(legacyDir, "bookmarks.yaml"), legacyContent, 0o644))

		newDir := filepath.Join(tmpDir, "state", "lfk")
		newPath := filepath.Join(newDir, "bookmarks.yaml")

		data := migrateStateFile("bookmarks.yaml", newPath)
		assert.Equal(t, legacyContent, data)

		// Verify new file was created.
		newData, err := os.ReadFile(newPath)
		require.NoError(t, err)
		assert.Equal(t, legacyContent, newData)

		// Verify legacy file was removed.
		_, err = os.Stat(filepath.Join(legacyDir, "bookmarks.yaml"))
		assert.True(t, os.IsNotExist(err))
	})
}

// --- loadSession ---

func TestLoadSessionNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	session := loadSession()
	assert.Nil(t, session)
}

// --- filter + cursor persistence ---

// The list filter and highlighted row must survive a quit/relaunch so the user
// reopens lfk exactly where they left off.
func TestSession_FilterAndCursorRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	in := SessionState{
		Context: "ctx",
		Tabs: []SessionTab{{
			Context:         "ctx",
			AllNamespaces:   true,
			ResourceType:    "v1/pods",
			Filter:          "web",
			FilterBroad:     true,
			CursorName:      "pod-2",
			CursorNamespace: "ns-2",
		}},
	}
	require.NoError(t, saveSession(in))

	out := loadSession()
	require.NotNil(t, out)
	require.Len(t, out.Tabs, 1)
	assert.Equal(t, "web", out.Tabs[0].Filter)
	assert.True(t, out.Tabs[0].FilterBroad)
	assert.Equal(t, "pod-2", out.Tabs[0].CursorName)
	assert.Equal(t, "ns-2", out.Tabs[0].CursorNamespace)
}

// applySessionFilterAndCursor seeds the live filter, commits it to filterMemory
// (so a drill-out/back keeps it), and arms the cursor target for the upcoming
// resource load.
func TestApplySessionFilterAndCursor(t *testing.T) {
	t.Run("seeds filter and cursor target", func(t *testing.T) {
		m := basePush80Model()
		m.filterMemory = make(map[string]savedFilter)

		m.applySessionFilterAndCursor("web", true, "pod-2", "ns-2")

		assert.Equal(t, "web", m.filterText)
		assert.Equal(t, "web", m.filterInput.Value)
		assert.True(t, m.filterBroadMode)
		assert.False(t, m.filterActive)
		assert.Equal(t, "pod-2", m.pendingTarget)
		assert.Equal(t, "ns-2", m.pendingTargetNamespace)

		f, ok := m.filterMemory[m.navKey()]
		require.True(t, ok, "committed filter must land in filterMemory")
		assert.Equal(t, savedFilter{text: "web", broad: true}, f)
	})

	t.Run("empty filter clears memory and leaves cursor untargeted", func(t *testing.T) {
		m := basePush80Model()
		key := m.navKey()
		m.filterMemory = map[string]savedFilter{key: {text: "stale", broad: true}}

		m.applySessionFilterAndCursor("", false, "", "")

		assert.Empty(t, m.filterText)
		assert.False(t, m.filterBroadMode)
		_, ok := m.filterMemory[key]
		assert.False(t, ok, "empty filter must drop the stale memory entry")
		assert.Empty(t, m.pendingTarget)
		assert.Empty(t, m.pendingTargetNamespace)
	})
}

// applyPendingSessionList only fires when armed (deferred behind CRD discovery)
// and is otherwise a no-op.
func TestApplyPendingSessionList(t *testing.T) {
	t.Run("armed state applies then clears", func(t *testing.T) {
		m := basePush80Model()
		m.filterMemory = make(map[string]savedFilter)
		m.pendingSessionList = pendingSessionListState{
			armed:       true,
			filter:      "db",
			filterBroad: true,
			cursorName:  "pod-3",
			cursorNs:    "default",
		}

		m.applyPendingSessionList()

		assert.Equal(t, "db", m.filterText)
		assert.True(t, m.filterBroadMode)
		assert.Equal(t, "pod-3", m.pendingTarget)
		assert.Equal(t, "default", m.pendingTargetNamespace)
		assert.False(t, m.pendingSessionList.armed, "armed state must be consumed")
	})

	t.Run("unarmed is a no-op", func(t *testing.T) {
		m := basePush80Model()
		m.filterText = "keep"
		m.applyPendingSessionList()
		assert.Equal(t, "keep", m.filterText)
		assert.Empty(t, m.pendingTarget)
	})
}

// sessionCursor folds a drilled-in ResourceName into the cursor target (name
// only, since its namespace was never persisted) and otherwise uses the saved
// cursor name/namespace.
func TestSessionCursor(t *testing.T) {
	t.Run("ResourceName wins, name only", func(t *testing.T) {
		name, ns := sessionCursor(&SessionState{ResourceName: "my-app", CursorName: "other", CursorNamespace: "x"})
		assert.Equal(t, "my-app", name)
		assert.Empty(t, ns)
	})

	t.Run("falls back to cursor name and namespace", func(t *testing.T) {
		name, ns := sessionCursor(&SessionState{CursorName: "pod-2", CursorNamespace: "ns-2"})
		assert.Equal(t, "pod-2", name)
		assert.Equal(t, "ns-2", ns)
	})
}

// saveCurrentSession captures the active tab's filter and highlighted row when
// it is sitting on a resource list.
func TestSaveCurrentSession_CapturesFilterAndCursor(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	m := basePush80Model()
	m.allNamespaces = true
	m.filterText = "pod"
	m.filterInput.Set("pod")
	m.filterBroadMode = true
	// Land the cursor on the second visible row (pod-2 / ns-2).
	m.setCursor(1)

	m.saveCurrentSession()

	out := loadSession()
	require.NotNil(t, out)
	require.Len(t, out.Tabs, 1)
	assert.Equal(t, "pod", out.Tabs[0].Filter)
	assert.True(t, out.Tabs[0].FilterBroad)
	assert.Equal(t, "pod-2", out.Tabs[0].CursorName)
	assert.Equal(t, "ns-2", out.Tabs[0].CursorNamespace)
}

// A filter typed at the resource-types level is not a resource-list filter and
// must not be persisted as one.
func TestSaveCurrentSession_SkipsFilterAboveResourceLevel(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m.tabs[0].nav.Level = model.LevelResourceTypes
	m.filterText = "dep"
	m.filterInput.Set("dep")

	m.saveCurrentSession()

	out := loadSession()
	require.NotNil(t, out)
	require.Len(t, out.Tabs, 1)
	assert.Empty(t, out.Tabs[0].Filter)
	assert.Empty(t, out.Tabs[0].CursorName)
}
