package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/model"
)

// --- loadBookmarks / saveBookmarks ---

func TestSaveAndLoadBookmarks(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	bookmarks := []model.Bookmark{
		{
			Name:         "prod-pods",
			Context:      "prod-cluster",
			Namespace:    "production",
			ResourceType: "pods",
		},
		{
			Name:         "dev-deployments",
			Context:      "dev-cluster",
			Namespace:    "development",
			ResourceType: "deployments",
		},
	}

	err := saveBookmarks(bookmarks)
	require.NoError(t, err)

	// Verify file was created.
	expectedPath := filepath.Join(tmpDir, "lfk", "bookmarks.yaml")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err)

	// Load and verify.
	loaded := loadBookmarks()
	require.Len(t, loaded, 2)
	assert.Equal(t, "prod-pods", loaded[0].Name)
	assert.Equal(t, "prod-cluster", loaded[0].Context)
	assert.Equal(t, "production", loaded[0].Namespace)
	assert.Equal(t, "dev-deployments", loaded[1].Name)
}

func TestLoadBookmarksNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	loaded := loadBookmarks()
	assert.Nil(t, loaded)
}

func TestSaveBookmarksEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	err := saveBookmarks([]model.Bookmark{})
	require.NoError(t, err)

	loaded := loadBookmarks()
	assert.Empty(t, loaded)
}

func TestSaveBookmarksOverwrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Save initial bookmarks.
	err := saveBookmarks([]model.Bookmark{{Name: "first"}})
	require.NoError(t, err)

	// Overwrite with different bookmarks.
	err = saveBookmarks([]model.Bookmark{{Name: "second"}, {Name: "third"}})
	require.NoError(t, err)

	loaded := loadBookmarks()
	require.Len(t, loaded, 2)
	assert.Equal(t, "second", loaded[0].Name)
	assert.Equal(t, "third", loaded[1].Name)
}

// --- bookmarksFilePath ---

func TestBookmarksFilePath(t *testing.T) {
	t.Run("uses XDG_STATE_HOME when set", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		path := bookmarksFilePath()
		assert.Equal(t, "/custom/state/lfk/bookmarks.yaml", path)
	})

	t.Run("falls back to home directory", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		path := bookmarksFilePath()
		assert.Contains(t, path, ".local/state/lfk/bookmarks.yaml")
		assert.NotEmpty(t, path)
	})
}

// --- Context field YAML persistence ---

// TestBookmarkContextPersistence verifies that the Context field round-trips
// through save/load correctly and that the omitempty tag causes context-free
// bookmarks to not write an empty "context:" line.
func TestBookmarkContextPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	bookmarks := []model.Bookmark{
		{
			Name:         "context-aware-mark",
			Context:      "prod-cluster",
			Namespace:    "production",
			ResourceType: "/v1/pods",
			Slot:         "a",
		},
		{
			Name:         "context-free-mark",
			Namespace:    "development",
			ResourceType: "apps/v1/deployments",
			Slot:         "A",
		},
	}

	err := saveBookmarks(bookmarks)
	require.NoError(t, err)

	loaded := loadBookmarks()
	require.Len(t, loaded, 2)

	assert.True(t, loaded[0].IsContextAware(),
		"context-aware bookmark should report IsContextAware=true after reload")
	assert.Equal(t, "prod-cluster", loaded[0].Context)
	assert.Equal(t, "a", loaded[0].Slot)

	assert.False(t, loaded[1].IsContextAware(),
		"context-free bookmark should report IsContextAware=false after reload")
	assert.Empty(t, loaded[1].Context)
	assert.Equal(t, "A", loaded[1].Slot)

	rawYAML, err := yaml.Marshal(bookmarks)
	require.NoError(t, err)
	yamlStr := string(rawYAML)

	assert.Contains(t, yamlStr, "context: prod-cluster",
		"context-aware bookmark must write context field")
	assert.NotContains(t, yamlStr, "global:",
		"saved YAML must not contain legacy global field")

	// Note: omitempty verification for context-free entries lives in
	// TestSaveLoadRoundTripNewFormat (Task 7), which runs after Task 5
	// adds the omitempty tag to the Context field.
}

// TestLoadBookmarksLegacyGlobalField verifies that bookmark files written
// before the Global field was removed still load correctly. The unknown
// "global:" key is silently dropped by sigs.k8s.io/yaml; context-awareness
// is determined by the Context field presence.
func TestLoadBookmarksLegacyGlobalField(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	legacyYAML := `- name: legacy-context-aware
  context: test-cluster
  namespace: default
  resource_type: /v1/pods
  slot: A
  global: true
- name: legacy-context-free
  context: ""
  namespace: default
  resource_type: /v1/pods
  slot: a
  global: false
`

	path := filepath.Join(tmpDir, "lfk", "bookmarks.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(legacyYAML), 0o600))

	loaded := loadBookmarks()
	require.Len(t, loaded, 2)

	assert.Equal(t, "legacy-context-aware", loaded[0].Name)
	assert.Equal(t, "test-cluster", loaded[0].Context)
	assert.True(t, loaded[0].IsContextAware(),
		"legacy bookmark with populated context must load as context-aware")

	assert.Equal(t, "legacy-context-free", loaded[1].Name)
	assert.Empty(t, loaded[1].Context)
	assert.False(t, loaded[1].IsContextAware(),
		"legacy bookmark with empty context must load as context-free")
}

// TestSaveLoadRoundTripNewFormat verifies that saved bookmarks use the new
// schema (no global field, no empty context lines) and reload correctly.
func TestSaveLoadRoundTripNewFormat(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	original := []model.Bookmark{
		{
			Name:         "aware",
			Context:      "prod-cluster",
			Namespace:    "production",
			ResourceType: "/v1/pods",
			Slot:         "a",
		},
		{
			Name:         "free",
			Namespace:    "development",
			ResourceType: "apps/v1/deployments",
			Slot:         "A",
		},
	}

	require.NoError(t, saveBookmarks(original))

	rawBytes, err := os.ReadFile(filepath.Join(tmpDir, "lfk", "bookmarks.yaml"))
	require.NoError(t, err)
	raw := string(rawBytes)

	assert.NotContains(t, raw, "global:",
		"saved bookmark file must not contain the legacy global field")

	assert.Contains(t, raw, "context: prod-cluster",
		"context-aware bookmark must write its context")

	// Scan the raw YAML to verify the context-free entry (the second one,
	// entryIdx == 1) omits the "context:" line entirely thanks to omitempty.
	// The context-aware entry's "context: prod-cluster" line is validated
	// by the global assert.Contains above, so we only guard the
	// "must not contain" side here.
	// Track which YAML sequence entry we're in by counting "- " lines.
	entryIdx := -1
	for line := range strings.SplitSeq(raw, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "- ") {
			entryIdx++
		}
		// The second entry (index 1) is the context-free "free" bookmark.
		if entryIdx == 1 {
			assert.NotContains(t, line, "context:",
				"context-free bookmark with empty Context must omit the context field")
		}
	}
	require.Equal(t, 1, entryIdx, "expected exactly 2 YAML entries in saved bookmark file")

	loaded := loadBookmarks()
	require.Len(t, loaded, 2)

	assert.Equal(t, original[0].Name, loaded[0].Name)
	assert.Equal(t, original[0].Context, loaded[0].Context)
	assert.Equal(t, original[0].Namespace, loaded[0].Namespace)
	assert.Equal(t, original[0].ResourceType, loaded[0].ResourceType)
	assert.Equal(t, original[0].Slot, loaded[0].Slot)
	assert.True(t, loaded[0].IsContextAware())

	assert.Equal(t, original[1].Name, loaded[1].Name)
	assert.Empty(t, loaded[1].Context)
	assert.Equal(t, original[1].Namespace, loaded[1].Namespace)
	assert.Equal(t, original[1].ResourceType, loaded[1].ResourceType)
	assert.Equal(t, original[1].Slot, loaded[1].Slot)
	assert.False(t, loaded[1].IsContextAware())
}
