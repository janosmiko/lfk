package app

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMain redirects every config/state/data directory into a per-package
// temp sandbox so any test that reaches savePinnedState, saveBookmarks,
// saveSession, the log writer, or any other on-disk writer cannot overwrite
// the developer's real lfk files (e.g. resetting pinned resource types).
//
// It UNSETS the LFK_* variables and SETS the XDG_* variables. paths.resolve
// checks LFK_<X>_DIR before XDG_<X>_HOME, so a developer whose environment sets
// LFK_STATE_DIR / LFK_CONFIG_DIR / LFK_DATA_DIR (portable installs, custom data
// dirs) would otherwise bypass an XDG-only sandbox and have tests write to
// their REAL files. Unsetting LFK_* (rather than pointing it at the sandbox)
// is deliberate: it keeps XDG_* as the effective resolver, so individual tests
// that isolate themselves with t.Setenv("XDG_STATE_HOME", t.TempDir()) keep
// working — pointing LFK_* at the sandbox would override those per-test XDG
// overrides and cross-contaminate tests.
func TestMain(m *testing.M) {
	os.Exit(runTests(m))
}

// runTests is a helper so deferred cleanup actually runs (a defer in TestMain
// alongside os.Exit would never fire).
func runTests(m *testing.M) int {
	tmp, err := os.MkdirTemp("", "lfk-app-tests-")
	if err != nil {
		panic("test setup: cannot create temp sandbox dir: " + err.Error())
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	// Drop any inherited LFK_* so they can't take precedence over the XDG
	// sandbox below (and can't point tests at the developer's real files).
	for _, k := range []string{"LFK_CONFIG_DIR", "LFK_STATE_DIR", "LFK_DATA_DIR"} {
		if err := os.Unsetenv(k); err != nil {
			panic("test setup: cannot unset " + k + ": " + err.Error())
		}
	}
	// XDG_* now resolves everything into the sandbox (paths.resolve appends an
	// "lfk" component). Code that reads XDG_STATE_HOME directly (e.g.
	// security_ignores) is covered too.
	xdg := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(tmp, "config"),
		"XDG_STATE_HOME":  filepath.Join(tmp, "state"),
		"XDG_DATA_HOME":   filepath.Join(tmp, "data"),
	}
	for k, v := range xdg {
		if err := os.Setenv(k, v); err != nil {
			panic("test setup: cannot set " + k + ": " + err.Error())
		}
	}

	return m.Run()
}
