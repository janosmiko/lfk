package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// isolateViewerPrefs points the state directory at a fresh temp dir and restores
// every viewer global afterwards, so one case cannot leak into the next.
// It returns the path the prefs file resolves to.
func isolateViewerPrefs(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	// LFK_STATE_DIR outranks XDG_STATE_HOME in paths.resolve. TestMain unsets it
	// for the package, but pinning it here keeps a stray value from sending a
	// persistViewerPref write to the developer's real state directory.
	t.Setenv("LFK_STATE_DIR", filepath.Join(dir, "lfk"))

	saved := make([]bool, len(viewerPrefBindings))
	for i, b := range viewerPrefBindings {
		saved[i] = *b.global
	}
	t.Cleanup(func() {
		for i, b := range viewerPrefBindings {
			*b.global = saved[i]
		}
	})
	return filepath.Join(dir, "lfk", "viewer_prefs.yaml")
}

// TestViewerPrefBindings_CoverEveryPref catches the way this file goes wrong:
// a new toggle added to the viewerPref list but not to the binding table would
// otherwise index past the table and panic at the first keypress.
func TestViewerPrefBindings_CoverEveryPref(t *testing.T) {
	if len(viewerPrefBindings) != int(numViewerPrefs) {
		t.Fatalf("viewerPrefBindings has %d entries, viewerPref has %d",
			len(viewerPrefBindings), numViewerPrefs)
	}
}

func TestApplyViewerPrefs_MissingFileKeepsConfigDefaults(t *testing.T) {
	isolateViewerPrefs(t)

	ui.ConfigLogWrap = true
	ui.ConfigLogShowTimestamps = true
	ui.ConfigObjectExplorerLive = false

	ApplyViewerPrefs()

	if !ui.ConfigLogWrap || !ui.ConfigLogShowTimestamps || ui.ConfigObjectExplorerLive {
		t.Fatalf("missing prefs file must not touch config defaults: wrap=%v ts=%v live=%v",
			ui.ConfigLogWrap, ui.ConfigLogShowTimestamps, ui.ConfigObjectExplorerLive)
	}
}

func TestApplyViewerPrefs_CorruptFileKeepsConfigDefaults(t *testing.T) {
	path := isolateViewerPrefs(t)
	writeViewerPrefsFile(t, path, "log_wrap: [not, a, bool")

	ui.ConfigLogWrap = true
	ApplyViewerPrefs()

	if !ui.ConfigLogWrap {
		t.Fatal("corrupt prefs file must fall back to the config default")
	}
}

func TestApplyViewerPrefs_AbsentFieldKeepsConfigDefault(t *testing.T) {
	path := isolateViewerPrefs(t)
	// Only log_wrap was ever toggled; describe_viewer_wrap keeps its default.
	writeViewerPrefsFile(t, path, "log_wrap: true\n")

	ui.ConfigLogWrap = false
	ui.ConfigDescribeViewerWrap = true

	ApplyViewerPrefs()

	if !ui.ConfigLogWrap {
		t.Error("log_wrap: true in the prefs file must override the config default")
	}
	if !ui.ConfigDescribeViewerWrap {
		t.Error("an absent field must leave the config default alone")
	}
}

// TestPersistViewerPref_WritesTheFileNotTheGlobal pins the split that keeps the
// rest of the suite order-independent: a keypress touches disk only, and
// ApplyViewerPrefs is the one writer of the startup seed.
func TestPersistViewerPref_WritesTheFileNotTheGlobal(t *testing.T) {
	path := isolateViewerPrefs(t)

	ui.ConfigLogWrap = false
	persistViewerPref(prefLogWrap, true)

	if ui.ConfigLogWrap {
		t.Error("a keypress must not rewrite the startup seed")
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("persistViewerPref must write the state file: %v", err)
	}

	ApplyViewerPrefs()
	if !ui.ConfigLogWrap {
		t.Error("the persisted value must survive a reload")
	}
}

// TestPersistViewerPref_EveryToggleRoundTrips guards the binding table: a field
// wired to the wrong global, or a global with no field, fails here.
func TestPersistViewerPref_EveryToggleRoundTrips(t *testing.T) {
	isolateViewerPrefs(t)

	// Alternate the values so a table entry pointing at its neighbour's field
	// produces a mismatch instead of an accidental pass.
	want := make([]bool, len(viewerPrefBindings))
	for i := range viewerPrefBindings {
		want[i] = i%2 == 0
		persistViewerPref(viewerPref(i), want[i])
	}

	for _, b := range viewerPrefBindings {
		*b.global = false
	}
	ApplyViewerPrefs()

	for i, b := range viewerPrefBindings {
		if *b.global != want[i] {
			t.Errorf("binding %d: got %v, want %v after round trip", i, *b.global, want[i])
		}
	}
}

// TestPersistViewerPref_MergesWithExistingFile proves one toggle never drops
// another already on disk.
func TestPersistViewerPref_MergesWithExistingFile(t *testing.T) {
	isolateViewerPrefs(t)

	persistViewerPref(prefLogWrap, true)
	persistViewerPref(prefDiffViewerUnified, true)

	ui.ConfigLogWrap = false
	ui.ConfigDiffViewerUnified = false
	ApplyViewerPrefs()

	if !ui.ConfigLogWrap {
		t.Error("the second toggle dropped the first from the state file")
	}
	if !ui.ConfigDiffViewerUnified {
		t.Error("the second toggle was not persisted")
	}
}

func writeViewerPrefsFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
