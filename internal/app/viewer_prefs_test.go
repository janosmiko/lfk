package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/yaml"

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

// isolateMetricsSparkPref points the state directory at a fresh temp dir and
// restores ui.MetricsSparkStartupState and ui.ConfigSparklineWindows, so one
// case cannot leak into the next under -shuffle=on.
func isolateMetricsSparkPref(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	t.Setenv("LFK_STATE_DIR", filepath.Join(dir, "lfk"))

	prevSeed := ui.MetricsSparkStartupState
	prevWindows := ui.ConfigSparklineWindows
	t.Cleanup(func() {
		ui.MetricsSparkStartupState = prevSeed
		ui.ConfigSparklineWindows = prevWindows
	})
}

func TestApplyViewerPrefs_MetricsSparkWindow_RoundTrips(t *testing.T) {
	isolateMetricsSparkPref(t)
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute, time.Hour}

	persistMetricsSparkPref(ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark, WindowIdx: 1})
	ui.MetricsSparkStartupState = ui.MetricsSparkState{}

	ApplyViewerPrefs()

	assert.Equal(t, ui.MetricsDisplaySpark, ui.MetricsSparkStartupState.Mode)
	assert.Equal(t, 15*time.Minute, ui.MetricsSparkStartupState.Window())
}

// A persisted numeric choice must overwrite a stale sparkline seed on
// reload, which only happens if it round-tripped as explicit, not "never touched".
func TestApplyViewerPrefs_MetricsSparkNumeric_RoundTripsAsExplicitNumeric(t *testing.T) {
	isolateMetricsSparkPref(t)
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute}

	persistMetricsSparkPref(ui.MetricsSparkState{})
	ui.MetricsSparkStartupState = ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark, WindowIdx: 0}

	ApplyViewerPrefs()

	assert.Equal(t, ui.MetricsDisplayNumeric, ui.MetricsSparkStartupState.Mode)
}

func TestApplyViewerPrefs_MetricsSparkWindow_NoLongerConfiguredFallsBackToNumeric(t *testing.T) {
	isolateMetricsSparkPref(t)
	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute, 15 * time.Minute}

	persistMetricsSparkPref(ui.MetricsSparkState{Mode: ui.MetricsDisplaySpark, WindowIdx: 1})

	ui.ConfigSparklineWindows = []time.Duration{5 * time.Minute}
	ApplyViewerPrefs()

	assert.Equal(t, ui.MetricsDisplayNumeric, ui.MetricsSparkStartupState.Mode)
}

func TestApplyViewerPrefs_MetricsSparkNeverTouched_LeavesNumericWithoutError(t *testing.T) {
	isolateMetricsSparkPref(t)

	assert.NotPanics(t, ApplyViewerPrefs)
	assert.Equal(t, ui.MetricsDisplayNumeric, ui.MetricsSparkStartupState.Mode)
}

// Pins the mechanism rule 3 depends on: nil is omitted (never touched) while
// a pointer to "" still encodes (explicit numeric), and both must decode
// back to their original shape.
func TestViewerPrefsState_MetricsSparkWindow_NilVsEmptyStringSurviveRoundTrip(t *testing.T) {
	untouched := ViewerPrefsState{}
	data, err := yaml.Marshal(untouched)
	require.NoError(t, err)
	assert.NotContains(t, string(data), "metrics_spark_window")

	empty := ""
	explicitNumeric := ViewerPrefsState{MetricsSparkWindow: &empty}
	data, err = yaml.Marshal(explicitNumeric)
	require.NoError(t, err)
	assert.Contains(t, string(data), "metrics_spark_window")

	var back ViewerPrefsState
	require.NoError(t, yaml.Unmarshal(data, &back))
	require.NotNil(t, back.MetricsSparkWindow)
	assert.Equal(t, "", *back.MetricsSparkWindow)
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
