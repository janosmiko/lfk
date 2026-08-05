package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/janosmiko/lfk/internal/ui"
)

// isolateStateDir gives one test its own state directory so the on-disk
// which-key preference cannot leak into (or out of) the package sandbox.
func isolateStateDir(t *testing.T) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
}

func TestWhichKeyPrefs_MissingFileIsNoPreference(t *testing.T) {
	isolateStateDir(t)
	if got := loadWhichKeyGrouping(); got != wkGroupDefault {
		t.Fatalf("no state file must mean no preference, got %v", got)
	}
}

func TestWhichKeyPrefs_CorruptFileIsNoPreference(t *testing.T) {
	isolateStateDir(t)
	path := whichKeyPrefsFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("grouped: [not a bool\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadWhichKeyGrouping(); got != wkGroupDefault {
		t.Fatalf("a corrupt state file must mean no preference, got %v", got)
	}
}

func TestWhichKeyPrefs_RoundTripsTheRuntimeChoice(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyGrouped = true

	saveWhichKeyGrouping(false)
	if got := loadWhichKeyGrouping(); got != wkGroupOff {
		t.Fatalf("saved grouped=false, loaded %v", got)
	}
	saveWhichKeyGrouping(true)
	if got := loadWhichKeyGrouping(); got != wkGroupOn {
		t.Fatalf("saved grouped=true, loaded %v", got)
	}
}

// The persisted runtime choice outranks the config default, which is only a
// STARTUP default.
func TestWhichKeyPrefs_StateOutranksTheConfigDefault(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyGrouped = true
	saveWhichKeyGrouping(false)
	if got := loadWhichKeyGrouping(); got != wkGroupOff {
		t.Fatalf("state must win over which_key_grouped: true, got %v", got)
	}
}

// A config edit made AFTER the toggle retires the stale choice: the recorded
// default no longer matches, so the newer config value wins.
func TestWhichKeyPrefs_ConfigEditedAfterTheToggleRetiresTheState(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyGrouped = true
	saveWhichKeyGrouping(false)

	ui.ConfigWhichKeyGrouped = false // user edits config.yaml afterwards
	if got := loadWhichKeyGrouping(); got != wkGroupDefault {
		t.Fatalf("a changed which_key_grouped must retire the stale state, got %v", got)
	}
}

// A hand-written file with no config_default is honoured as-is: the staleness
// check only fires when there is a recorded baseline to compare against.
func TestWhichKeyPrefs_HandWrittenFileWithoutBaselineIsHonoured(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ConfigWhichKeyGrouped = true
	path := whichKeyPrefsFilePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("grouped: false\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := loadWhichKeyGrouping(); got != wkGroupOff {
		t.Fatalf("a baseline-less file must still be honoured, got %v", got)
	}
}

// The toggle itself writes, so a kill -9 straight after pressing the leader
// key still leaves the choice on disk.
func TestToggleWhichKeyGrouping_PersistsImmediately(t *testing.T) {
	restoreWhichKeyGlobals(t)
	ui.ActiveKeybindings = ui.DefaultKeybindings()
	ui.ConfigWhichKeyGrouped = true

	m := whichKeyTestModel().toggleWhichKeyGrouping()
	if m.whichKeyGrouped() {
		t.Fatal("precondition: the toggle must have flipped to key order")
	}
	if got := loadWhichKeyGrouping(); got != wkGroupOff {
		t.Fatalf("the toggle must write through to disk, got %v", got)
	}
}

func TestWhichKeyPrefs_FileIsOwnerOnly(t *testing.T) {
	restoreWhichKeyGlobals(t)
	saveWhichKeyGrouping(false)
	fi, err := os.Stat(whichKeyPrefsFilePath())
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Fatalf("state file mode is %v, want 0600", got)
	}
}
