package app

import (
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
)

// --- filteredBookmarks returns a defensive copy ---

func TestFilteredBookmarks_UnfilteredReturnsCopy(t *testing.T) {
	bookmarks := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
		{Slot: "c", Name: "charlie"},
	}
	m := Model{
		bookmarks:      bookmarks,
		bookmarkFilter: TextInput{},
	}

	result := m.filteredBookmarks()

	// The returned slice must NOT be the same backing array as m.bookmarks.
	// Mutating the result must not affect m.bookmarks.
	require.Len(t, result, 3)
	result[0] = model.Bookmark{Slot: "z", Name: "mutated"}

	assert.Equal(t, "a", m.bookmarks[0].Slot,
		"mutating filteredBookmarks result must not affect the model's bookmarks")
	assert.Equal(t, "alpha", m.bookmarks[0].Name)
}

// --- bookmarkDeleteCurrent through value receiver chain ---

func TestDeleteBookmark_ValueReceiverChain_PersistsToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	bookmarks := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
		{Slot: "c", Name: "charlie"},
	}
	// Pre-save bookmarks to disk.
	require.NoError(t, saveBookmarks(bookmarks))

	m := Model{
		bookmarks:          bookmarks,
		bookmarkFilter:     TextInput{},
		bookmarkSearchMode: bookmarkModeConfirmDelete,
		overlayCursor:      1, // pointing at "bravo"
		overlay:            overlayBookmarks,
		tabs:               []TabState{{}},
	}

	// Simulate the full value-receiver chain: handleBookmarkOverlayKey -> handleBookmarkConfirmDelete -> bookmarkDeleteCurrent.
	yesKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	result, _ := m.handleBookmarkOverlayKey(yesKey)
	resultModel := result.(Model)

	// In-memory: bravo should be gone.
	require.Len(t, resultModel.bookmarks, 2)
	assert.Equal(t, "a", resultModel.bookmarks[0].Slot)
	assert.Equal(t, "c", resultModel.bookmarks[1].Slot)

	// On disk: should also have only 2 bookmarks.
	diskBookmarks := loadBookmarks()
	require.Len(t, diskBookmarks, 2, "disk should reflect the deletion")
	assert.Equal(t, "a", diskBookmarks[0].Slot)
	assert.Equal(t, "c", diskBookmarks[1].Slot)
}

// --- bookmarkDeleteAll through value receiver chain ---

func TestDeleteAllBookmarks_ValueReceiverChain_PersistsToDisk(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	bookmarks := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
	}
	require.NoError(t, saveBookmarks(bookmarks))

	m := Model{
		bookmarks:          bookmarks,
		bookmarkFilter:     TextInput{},
		bookmarkSearchMode: bookmarkModeConfirmDeleteAll,
		overlay:            overlayBookmarks,
		tabs:               []TabState{{}},
	}

	yesKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	result, _ := m.handleBookmarkOverlayKey(yesKey)
	resultModel := result.(Model)

	assert.Empty(t, resultModel.bookmarks, "in-memory bookmarks should be empty")

	diskBookmarks := loadBookmarks()
	assert.Empty(t, diskBookmarks, "disk bookmarks should be empty after delete-all")
}

// --- atomic write: partial write doesn't lose data ---

func TestSaveBookmarks_AtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Save initial bookmarks.
	initial := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
	}
	require.NoError(t, saveBookmarks(initial))

	// Verify file exists and has content.
	path := bookmarksFilePath()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	// Overwrite with new bookmarks.
	updated := []model.Bookmark{
		{Slot: "a", Name: "alpha-updated"},
		{Slot: "b", Name: "bravo"},
		{Slot: "c", Name: "charlie"},
	}
	require.NoError(t, saveBookmarks(updated))

	// Verify overwrite succeeded.
	loaded := loadBookmarks()
	require.Len(t, loaded, 3)
	assert.Equal(t, "alpha-updated", loaded[0].Name)
}

// --- loadBookmarks handles zero-byte file ---

func TestLoadBookmarks_ZeroByteFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Create a zero-byte file (simulates interrupted write).
	path := filepath.Join(tmpDir, "lfk", "bookmarks.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))

	loaded := loadBookmarks()
	assert.Nil(t, loaded, "zero-byte file should be treated as no bookmarks")
}

// --- saveBookmarks writes nil as clean empty ---

func TestSaveBookmarks_NilWritesCleanFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// First save some bookmarks.
	require.NoError(t, saveBookmarks([]model.Bookmark{{Slot: "a"}}))
	loaded := loadBookmarks()
	require.Len(t, loaded, 1)

	// Now save nil (delete all).
	require.NoError(t, saveBookmarks(nil))

	// Loading should return nil, not error.
	loaded = loadBookmarks()
	assert.Nil(t, loaded)
}

// --- filteredBookmarks with filter also returns independent copy ---

func TestFilteredBookmarks_FilteredReturnsCopy(t *testing.T) {
	bookmarks := []model.Bookmark{
		{Slot: "a", Name: "alpha-prod"},
		{Slot: "b", Name: "bravo-dev"},
		{Slot: "c", Name: "charlie-prod"},
	}
	m := Model{
		bookmarks:      bookmarks,
		bookmarkFilter: TextInput{Value: "prod"},
	}

	result := m.filteredBookmarks()
	require.Len(t, result, 2)

	// Mutating result must not affect m.bookmarks.
	result[0] = model.Bookmark{Slot: "z", Name: "mutated"}
	assert.Equal(t, "a", m.bookmarks[0].Slot)
	assert.Equal(t, "alpha-prod", m.bookmarks[0].Name)
}

// --- multiple sequential saves don't lose data ---

func TestSequentialSaves_NoDataLoss(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	rt := podResourceType()

	m := Model{
		nav: model.NavigationState{
			Level:        model.LevelResources,
			Context:      "test",
			ResourceType: rt,
		},
		namespace: "default",
		tabs:      []TabState{{}},
	}

	// Save bookmark 'a'.
	result, _ := m.saveBookmark(model.Bookmark{Slot: "a", Name: "alpha"})
	m = result.(Model)

	// Save bookmark 'b'.
	result, _ = m.saveBookmark(model.Bookmark{Slot: "b", Name: "bravo"})
	m = result.(Model)

	// Save bookmark 'c'.
	result, _ = m.saveBookmark(model.Bookmark{Slot: "c", Name: "charlie"})
	m = result.(Model)

	// In-memory should have 3 bookmarks.
	require.Len(t, m.bookmarks, 3)

	// Disk should also have 3 bookmarks.
	loaded := loadBookmarks()
	require.Len(t, loaded, 3, "sequential saves must not lose earlier bookmarks")
	assert.Equal(t, "a", loaded[0].Slot)
	assert.Equal(t, "b", loaded[1].Slot)
	assert.Equal(t, "c", loaded[2].Slot)
}

// --- delete one from three, then save new, total should be 3 ---

func TestDeleteThenSave_NoCrossContamination(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	bookmarks := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
		{Slot: "c", Name: "charlie"},
	}
	require.NoError(t, saveBookmarks(bookmarks))

	m := Model{
		bookmarks:          bookmarks,
		bookmarkFilter:     TextInput{},
		bookmarkSearchMode: bookmarkModeConfirmDelete,
		overlayCursor:      1, // delete "bravo"
		overlay:            overlayBookmarks,
		tabs:               []TabState{{}},
	}

	// Delete bravo.
	yesKey := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}}
	result, _ := m.handleBookmarkOverlayKey(yesKey)
	m = result.(Model)
	require.Len(t, m.bookmarks, 2)

	// Now save a new bookmark 'd'.
	result, _ = m.saveBookmark(model.Bookmark{Slot: "d", Name: "delta"})
	m = result.(Model)

	// Should have [a, c, d].
	require.Len(t, m.bookmarks, 3)

	// Verify disk.
	loaded := loadBookmarks()
	require.Len(t, loaded, 3, "delete + save should result in 3 bookmarks on disk")

	slotSet := map[string]bool{}
	for _, bm := range loaded {
		slotSet[bm.Slot] = true
	}
	assert.True(t, slotSet["a"])
	assert.False(t, slotSet["b"], "deleted bookmark should not be on disk")
	assert.True(t, slotSet["c"])
	assert.True(t, slotSet["d"])
}

// --- backup recovery when primary file is corrupt ---

func TestLoadBookmarks_RecoverFromBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Save bookmarks (this also creates a backup on second save).
	initial := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
	}
	require.NoError(t, saveBookmarks(initial))

	// Save again so the backup contains the first version.
	updated := []model.Bookmark{
		{Slot: "a", Name: "alpha"},
		{Slot: "b", Name: "bravo"},
		{Slot: "c", Name: "charlie"},
	}
	require.NoError(t, saveBookmarks(updated))

	// Corrupt the primary file.
	path := bookmarksFilePath()
	require.NoError(t, os.WriteFile(path, []byte("!!!invalid yaml{{{"), 0o644))

	// Load should recover from backup (which has a, b).
	loaded := loadBookmarks()
	require.NotNil(t, loaded, "should recover bookmarks from backup")
	assert.Len(t, loaded, 2, "backup had 2 bookmarks before the third was added")
}

// --- backup is created on save ---

func TestSaveBookmarks_CreatesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	// Save initial bookmarks.
	require.NoError(t, saveBookmarks([]model.Bookmark{{Slot: "a", Name: "alpha"}}))

	// Save updated bookmarks — this should backup the previous version.
	require.NoError(t, saveBookmarks([]model.Bookmark{{Slot: "b", Name: "bravo"}}))

	// Backup file should exist and contain the old data.
	bakPath := bookmarksFilePath() + ".bak"
	bakData, err := os.ReadFile(bakPath)
	require.NoError(t, err, "backup file should exist")
	assert.Contains(t, string(bakData), "alpha", "backup should contain previous bookmarks")
}

// --- empty save removes backup to prevent resurrection ---

func TestSaveBookmarks_EmptyRemovesBackup(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	require.NoError(t, saveBookmarks([]model.Bookmark{{Slot: "a"}}))
	require.NoError(t, saveBookmarks([]model.Bookmark{{Slot: "b"}}))

	// Backup should exist now.
	bakPath := bookmarksFilePath() + ".bak"
	_, err := os.Stat(bakPath)
	require.NoError(t, err, "backup should exist after two saves")

	// Save empty (delete all) should remove backup.
	require.NoError(t, saveBookmarks(nil))
	_, err = os.Stat(bakPath)
	assert.True(t, os.IsNotExist(err), "backup should be removed after saving empty")
}
