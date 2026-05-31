package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- pinnedFilePath ---

func TestPinnedFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		path := pinnedFilePath()
		assert.Equal(t, "/custom/state/lfk/pinned.yaml", path)
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		path := pinnedFilePath()
		assert.Contains(t, path, ".local/state/lfk/pinned.yaml")
	})
}

// --- togglePinnedType ---

func TestTogglePinnedGroup(t *testing.T) {
	t.Run("pins a new group", func(t *testing.T) {
		s := &PinnedState{Contexts: make(map[string][]string)}
		pinned := togglePinnedType(s, "prod", "argoproj.io")
		assert.True(t, pinned)
		assert.Equal(t, []string{"argoproj.io"}, s.Contexts["prod"])
	})

	t.Run("unpins an existing group", func(t *testing.T) {
		s := &PinnedState{
			Contexts: map[string][]string{
				"prod": {"argoproj.io", "fluxcd.io"},
			},
		}
		pinned := togglePinnedType(s, "prod", "argoproj.io")
		assert.False(t, pinned)
		assert.Equal(t, []string{"fluxcd.io"}, s.Contexts["prod"])
	})

	t.Run("pin and unpin idempotent", func(t *testing.T) {
		s := &PinnedState{Contexts: make(map[string][]string)}

		togglePinnedType(s, "dev", "my.group")
		assert.Len(t, s.Contexts["dev"], 1)

		togglePinnedType(s, "dev", "my.group")
		assert.Empty(t, s.Contexts["dev"])
	})

	t.Run("different contexts are independent", func(t *testing.T) {
		s := &PinnedState{Contexts: make(map[string][]string)}

		togglePinnedType(s, "prod", "group-a")
		togglePinnedType(s, "dev", "group-b")

		assert.Equal(t, []string{"group-a"}, s.Contexts["prod"])
		assert.Equal(t, []string{"group-b"}, s.Contexts["dev"])
	})
}

func TestTogglePinnedUnionSetGroup(t *testing.T) {
	s := &PinnedState{UnionSets: make(map[string][]string)}

	pinned := togglePinnedUnionSetType(s, "staging-west", "argoproj.io")
	assert.True(t, pinned)
	assert.Equal(t, []string{"argoproj.io"}, s.UnionSets["staging-west"])

	pinned = togglePinnedUnionSetType(s, "staging-west", "argoproj.io")
	assert.False(t, pinned)
	assert.Empty(t, s.UnionSets["staging-west"])
}

// --- savePinnedState / loadPinnedState ---

func TestSaveAndLoadPinnedState(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	s := &PinnedState{
		Contexts: map[string][]string{
			"prod": {"argoproj.io", "fluxcd.io"},
			"dev":  {"my-crd.io"},
		},
		UnionSets: map[string][]string{
			"staging-west": {"monitoring.coreos.com"},
		},
	}

	err := savePinnedState(s)
	require.NoError(t, err)

	// Verify file exists.
	path := filepath.Join(tmpDir, "lfk", "pinned.yaml")
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load and verify.
	loaded := loadPinnedState()
	assert.Equal(t, []string{"argoproj.io", "fluxcd.io"}, loaded.Contexts["prod"])
	assert.Equal(t, []string{"my-crd.io"}, loaded.Contexts["dev"])
	assert.Equal(t, []string{"monitoring.coreos.com"}, loaded.UnionSets["staging-west"])
}

func TestLoadPinnedStateNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	loaded := loadPinnedState()
	assert.NotNil(t, loaded.Contexts)
	assert.NotNil(t, loaded.UnionSets)
	assert.Empty(t, loaded.Contexts)
	assert.Empty(t, loaded.UnionSets)
}
