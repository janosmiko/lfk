package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- commandHistory.add ---

func TestCommandHistoryAdd(t *testing.T) {
	h := &commandHistory{cursor: -1}

	h.add("get pods")
	assert.Equal(t, []string{"get pods"}, h.entries)

	h.add("get deployments")
	assert.Equal(t, []string{"get pods", "get deployments"}, h.entries)
}

func TestCommandHistoryAddIgnoresEmpty(t *testing.T) {
	h := &commandHistory{cursor: -1}

	h.add("")
	assert.Empty(t, h.entries)

	h.add("   ")
	assert.Empty(t, h.entries)
}

func TestCommandHistoryAddDeduplicates(t *testing.T) {
	h := &commandHistory{cursor: -1}

	h.add("get pods")
	h.add("get pods")
	assert.Len(t, h.entries, 1)

	h.add("get deployments")
	h.add("get pods") // different from last entry, so not deduplicated
	assert.Len(t, h.entries, 3)
}

func TestCommandHistoryAddTrimsToMax(t *testing.T) {
	h := &commandHistory{cursor: -1}
	for i := range maxHistoryEntries + 10 {
		h.add("cmd-" + string(rune('a'+i%26)) + string(rune('0'+i/26)))
	}
	assert.LessOrEqual(t, len(h.entries), maxHistoryEntries)
}

// --- commandHistory.up ---

func TestCommandHistoryUp(t *testing.T) {
	h := &commandHistory{cursor: -1}
	h.entries = []string{"first", "second", "third"}

	// First up: should save draft and return last entry.
	result := h.up("current draft")
	assert.Equal(t, "third", result)
	assert.Equal(t, "current draft", h.draft)
	assert.Equal(t, 2, h.cursor)

	// Second up: returns previous entry.
	result = h.up("")
	assert.Equal(t, "second", result)
	assert.Equal(t, 1, h.cursor)

	// Third up: returns first entry.
	result = h.up("")
	assert.Equal(t, "first", result)
	assert.Equal(t, 0, h.cursor)

	// Already at beginning: stays there.
	result = h.up("")
	assert.Equal(t, "first", result)
	assert.Equal(t, 0, h.cursor)
}

func TestCommandHistoryUpEmpty(t *testing.T) {
	h := &commandHistory{cursor: -1}

	result := h.up("my input")
	assert.Equal(t, "my input", result)
}

// --- commandHistory.down ---

func TestCommandHistoryDown(t *testing.T) {
	h := &commandHistory{cursor: -1}
	h.entries = []string{"first", "second"}

	// Navigate up first.
	h.up("draft text")
	h.up("") // at "first"
	assert.Equal(t, 0, h.cursor)

	// Down returns "second".
	result := h.down()
	assert.Equal(t, "second", result)
	assert.Equal(t, 1, h.cursor)

	// Down past end restores draft.
	result = h.down()
	assert.Equal(t, "draft text", result)
	assert.Equal(t, -1, h.cursor)
}

func TestCommandHistoryDownNotBrowsing(t *testing.T) {
	h := &commandHistory{cursor: -1, draft: "my draft"}

	result := h.down()
	assert.Equal(t, "my draft", result)
}

// --- commandHistory.reset ---

func TestCommandHistoryReset(t *testing.T) {
	h := &commandHistory{cursor: 2, draft: "something"}

	h.reset()
	assert.Equal(t, -1, h.cursor)
	assert.Empty(t, h.draft)
}

// --- commandHistory.leaveBrowse ---

// leaveBrowse exits browse mode but preserves draft so a subsequent
// Down past newest restores the user's original pre-recall input.
func TestCommandHistoryLeaveBrowse(t *testing.T) {
	h := &commandHistory{
		entries: []string{"a", "b"},
		cursor:  1,
		draft:   "pre-recall",
	}

	h.leaveBrowse()
	assert.Equal(t, -1, h.cursor)
	assert.Equal(t, "pre-recall", h.draft, "draft must be preserved")
}

func TestCommandHistoryLeaveBrowseNilReceiver(t *testing.T) {
	var h *commandHistory
	assert.NotPanics(t, func() { h.leaveBrowse() })
}

// --- commandHistory.save / loadCommandHistory ---

func TestCommandHistorySaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	h := &commandHistory{cursor: -1}
	h.add("get pods")
	h.add("get deployments")
	h.add("logs my-pod")

	h.save()

	// Verify file exists.
	path := filepath.Join(tmpDir, "lfk", "history")
	_, err := os.Stat(path)
	require.NoError(t, err)

	// Load and verify.
	loaded := loadCommandHistory()
	assert.Equal(t, []string{"get pods", "get deployments", "logs my-pod"}, loaded.entries)
	assert.Equal(t, -1, loaded.cursor)
}

// History files persist raw search queries that may contain sensitive
// fragments (tokens, emails). On shared hosts, default 0644/0755 modes
// would leak these to other local users. Save() must use 0600 for the
// file and 0700 for the parent directory.
func TestCommandHistorySaveUsesRestrictivePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	h := &commandHistory{cursor: -1}
	h.add("secret query")
	h.save()

	dirInfo, err := os.Stat(filepath.Join(tmpDir, "lfk"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), dirInfo.Mode().Perm(), "lfk dir must be user-only")

	fileInfo, err := os.Stat(filepath.Join(tmpDir, "lfk", "history"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), fileInfo.Mode().Perm(), "history file must be user-only")
}

func TestLoadCommandHistoryNoFile(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmpDir)

	loaded := loadCommandHistory()
	assert.Empty(t, loaded.entries)
	assert.Equal(t, -1, loaded.cursor)
}

// --- historyFilePathFor ---

func TestHistoryFilePathFor(t *testing.T) {
	t.Run("uses XDG_STATE_HOME", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/custom/state")
		path := historyFilePathFor(historyFileCommand)
		assert.Equal(t, "/custom/state/lfk/history", path)
	})

	t.Run("falls back to home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		path := historyFilePathFor(historyFileCommand)
		assert.Contains(t, path, ".local/state/lfk/history")
	})
}

func TestCovCommandHistoryAdd(t *testing.T) {
	h := &commandHistory{cursor: -1}

	h.add("ls")
	assert.Len(t, h.entries, 1)

	// Empty: ignore.
	h.add("")
	assert.Len(t, h.entries, 1)

	// Whitespace only: ignore.
	h.add("   ")
	assert.Len(t, h.entries, 1)

	// Duplicate: ignore.
	h.add("ls")
	assert.Len(t, h.entries, 1)

	h.add("pwd")
	assert.Len(t, h.entries, 2)
}

func TestCovCommandHistoryUpDown(t *testing.T) {
	h := &commandHistory{
		entries: []string{"first", "second", "third"},
		cursor:  -1,
	}

	// Up from current input.
	assert.Equal(t, "third", h.up("current"))
	assert.Equal(t, "current", h.draft)

	assert.Equal(t, "second", h.up("ignored"))
	assert.Equal(t, "first", h.up("ignored"))
	// At start: stays at first.
	assert.Equal(t, "first", h.up("ignored"))

	// Down.
	assert.Equal(t, "second", h.down())
	assert.Equal(t, "third", h.down())
	// Past end: restore draft.
	assert.Equal(t, "current", h.down())
	assert.Equal(t, -1, h.cursor)

	// Down when not browsing: returns draft (which was saved as "current").
	result := h.down()
	assert.Equal(t, "current", result)
}

func TestCovCommandHistoryUpEmpty(t *testing.T) {
	h := &commandHistory{cursor: -1}
	assert.Equal(t, "current", h.up("current"))
}

func TestCovCommandHistoryReset(t *testing.T) {
	h := &commandHistory{
		entries: []string{"a", "b"},
		cursor:  1,
		draft:   "draft",
	}
	h.reset()
	assert.Equal(t, -1, h.cursor)
	assert.Empty(t, h.draft)
}

func TestCovCommandHistorySaveLoad(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	h := &commandHistory{cursor: -1}
	h.add("test command 1")
	h.add("test command 2")
	h.save()

	h2 := loadCommandHistory()
	assert.Len(t, h2.entries, 2)
	assert.Equal(t, "test command 1", h2.entries[0])
	assert.Equal(t, "test command 2", h2.entries[1])
}

// --- loadInputHistory: per-name files stay isolated ---

// TestLoadInputHistoryIsolatesQueryFromCommand pins the one separation
// that still matters after `/` and `f` were merged into a single
// query-history: kubectl-shaped `:` command bar entries must not appear
// when recalling `/`/`f` queries. Without isolation, pressing Up in `/`
// would surface ":get pods" as a suggested resource-name pattern.
func TestLoadInputHistoryIsolatesQueryFromCommand(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	qh := loadInputHistory(historyFileQuery)
	qh.add("nginx")
	qh.save()

	ch := loadCommandHistory()
	ch.add(":get pods")
	ch.save()

	// Reload from disk and verify no cross-contamination between the two
	// distinct files.
	gotQuery := loadInputHistory(historyFileQuery)
	gotCmd := loadCommandHistory()

	assert.Equal(t, []string{"nginx"}, gotQuery.entries)
	assert.Equal(t, []string{":get pods"}, gotCmd.entries)
}

// TestLoadInputHistoryIsolatesLogSearchFromQuery pins the second
// separation that matters: the log viewer's `/` matches raw log lines
// (substring/regex over arbitrary text), not resource names. Pooling
// it with the explorer query history would surface kubernetes resource
// patterns when recalling in the log viewer, and log fragments when
// recalling in the explorer — both irrelevant to the user.
func TestLoadInputHistoryIsolatesLogSearchFromQuery(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	qh := loadInputHistory(historyFileQuery)
	qh.add("nginx")
	qh.save()

	lh := loadInputHistory(historyFileLogSearch)
	lh.add("ERROR connection refused")
	lh.save()

	gotQuery := loadInputHistory(historyFileQuery)
	gotLog := loadInputHistory(historyFileLogSearch)

	assert.Equal(t, []string{"nginx"}, gotQuery.entries)
	assert.Equal(t, []string{"ERROR connection refused"}, gotLog.entries)
}

// TestSavePreservesFilenameAcrossSaves verifies a loaded instance keeps
// writing back to the same file, rather than silently defaulting to
// "history" after the first save.
func TestSavePreservesFilenameAcrossSaves(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_STATE_HOME", tmp)

	h := loadInputHistory(historyFileQuery)
	h.add("first")
	h.save()
	h.add("second")
	h.save()

	// File must exist at the query-history path, NOT at "history".
	_, err := os.Stat(filepath.Join(tmp, "lfk", historyFileQuery))
	require.NoError(t, err, "query-history file must exist after save")

	_, err = os.Stat(filepath.Join(tmp, "lfk", historyFileCommand))
	assert.Error(t, err, "command history file must NOT be created by query saves")
}

// --- nil-safe methods ---

// Nil receiver tolerance lets test models that don't initialize history
// pointers exercise the filter/search Enter/Esc paths without panicking.
// This mirrors how the rest of the Model fields are partially-init in
// test factories — a regression here would fail dozens of tests at once.
func TestCommandHistoryNilSafe(t *testing.T) {
	var h *commandHistory

	assert.NotPanics(t, func() { h.add("anything") })
	assert.NotPanics(t, func() { h.save() })
	assert.NotPanics(t, func() { h.reset() })
	assert.Equal(t, "draft", h.up("draft"))
	assert.Empty(t, h.down())
}
