package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- hiddenTypesFilePath ---

func TestHiddenTypesFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		path := hiddenTypesFilePath()
		assert.Equal(t, "/custom/state/lfk/hidden_types.yaml", path)
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		path := hiddenTypesFilePath()
		assert.Contains(t, path, filepath.Join(".local", "state", "lfk", "hidden_types.yaml"))
	})
}

// --- toggleHiddenType ---

func TestToggleHiddenType(t *testing.T) {
	t.Run("hides a new type", func(t *testing.T) {
		s := newHiddenTypesState()
		hidden := toggleHiddenType(s, "prod", "networking.k8s.io/ingresses")
		assert.True(t, hidden)
		assert.Equal(t, []string{"networking.k8s.io/ingresses"}, s.Contexts["prod"])
	})

	t.Run("shows an already-hidden type", func(t *testing.T) {
		s := &HiddenTypesState{
			Contexts: map[string][]string{
				"prod": {"networking.k8s.io/ingresses", "/limitranges"},
			},
		}
		hidden := toggleHiddenType(s, "prod", "networking.k8s.io/ingresses")
		assert.False(t, hidden)
		assert.Equal(t, []string{"/limitranges"}, s.Contexts["prod"])
	})

	t.Run("hide and show idempotent", func(t *testing.T) {
		s := newHiddenTypesState()
		toggleHiddenType(s, "dev", "_helm/releases")
		assert.Len(t, s.Contexts["dev"], 1)
		toggleHiddenType(s, "dev", "_helm/releases")
		assert.Empty(t, s.Contexts["dev"])
	})

	t.Run("different contexts are independent", func(t *testing.T) {
		s := newHiddenTypesState()
		toggleHiddenType(s, "prod", "/limitranges")
		toggleHiddenType(s, "dev", "_helm/releases")
		assert.Equal(t, []string{"/limitranges"}, s.Contexts["prod"])
		assert.Equal(t, []string{"_helm/releases"}, s.Contexts["dev"])
	})

	t.Run("nil contexts map is initialized", func(t *testing.T) {
		s := &HiddenTypesState{}
		hidden := toggleHiddenType(s, "prod", "/limitranges")
		assert.True(t, hidden)
		assert.Equal(t, []string{"/limitranges"}, s.Contexts["prod"])
	})
}

func TestToggleHiddenUnionSetType(t *testing.T) {
	t.Run("hides and shows within a union set", func(t *testing.T) {
		s := newHiddenTypesState()
		hidden := toggleHiddenUnionSetType(s, "all-prod", "/limitranges")
		assert.True(t, hidden)
		assert.Equal(t, []string{"/limitranges"}, s.UnionSets["all-prod"])

		hidden = toggleHiddenUnionSetType(s, "all-prod", "/limitranges")
		assert.False(t, hidden)
		assert.Empty(t, s.UnionSets["all-prod"])
	})

	t.Run("context and union-set scopes are independent", func(t *testing.T) {
		s := newHiddenTypesState()
		toggleHiddenType(s, "prod", "/limitranges")
		toggleHiddenUnionSetType(s, "all-prod", "_helm/releases")
		assert.Equal(t, []string{"/limitranges"}, s.Contexts["prod"])
		assert.Equal(t, []string{"_helm/releases"}, s.UnionSets["all-prod"])
	})
}

// --- load / save round-trip ---

func TestHiddenTypesLoadSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)

	t.Run("missing file yields empty state", func(t *testing.T) {
		s := loadHiddenTypesState()
		require.NotNil(t, s)
		assert.Empty(t, s.Contexts)
		assert.Empty(t, s.UnionSets)
	})

	t.Run("save then load preserves entries", func(t *testing.T) {
		s := newHiddenTypesState()
		toggleHiddenType(s, "prod", "networking.k8s.io/ingresses")
		toggleHiddenType(s, "prod", "/limitranges")
		toggleHiddenUnionSetType(s, "all", "_helm/releases")
		require.NoError(t, saveHiddenTypesState(s))

		loaded := loadHiddenTypesState()
		assert.ElementsMatch(t, []string{"networking.k8s.io/ingresses", "/limitranges"}, loaded.Contexts["prod"])
		assert.Equal(t, []string{"_helm/releases"}, loaded.UnionSets["all"])
	})

	t.Run("corrupt file yields empty state", func(t *testing.T) {
		path := hiddenTypesFilePath()
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte("not: [valid: yaml"), 0o644))
		s := loadHiddenTypesState()
		require.NotNil(t, s)
		assert.NotNil(t, s.Contexts)
	})
}
