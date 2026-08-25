package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/flock"
)

// These tests prove each load-modify-save runs inside withStateFileLock, by
// holding the lock and checking the function writes nothing while it is held.
//
// A racing-goroutines test was tried first and does not work here. These save
// paths use os.WriteFile, so the window between the load and the save is a
// fraction of the fsync-backed one in viewer_prefs. Below about 40 writers the
// goroutines never interleave and the test passes with the lock removed, which
// makes it worthless as a guard. Above that, waiters exhaust the 250ms budget
// and skip their writes by design, so the test fails with the lock in place.
// No writer count satisfies both. Holding the lock tests the same property and
// is deterministic.

// holdStateLock takes the lock withStateFileLock uses for path, standing in for
// a second lfk process, and returns the release func.
func holdStateLock(t *testing.T, path string) func() {
	t.Helper()
	lockPath := filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".lock")
	held := flock.New(lockPath)
	if err := held.Lock(); err != nil {
		t.Fatalf("test setup: cannot take the lock: %v", err)
	}
	return func() {
		if err := held.Unlock(); err != nil {
			t.Fatalf("test teardown: cannot release the lock: %v", err)
		}
	}
}

// assertWritesOnlyUnlocked runs write() twice: once while another holder owns
// the lock, where it must leave the file untouched, and once after release,
// where landed() must report the change arrived.
func assertWritesOnlyUnlocked(t *testing.T, path string, write func(), landed func() bool) {
	t.Helper()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("test setup: seed write never reached %s: %v", path, err)
	}

	release := holdStateLock(t, path)
	write()
	during, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read while locked: %v", err)
	}
	if string(during) != string(before) {
		t.Error("the file changed while another holder had the lock, so the load-modify-save runs outside it")
	}
	release()

	write()
	if !landed() {
		t.Error("the write did not land once the lock was free")
	}
}

func TestPersistColumnPrefsEntry_WritesInsideTheLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	persistColumnPrefsEntry("ctx\x00Seed", persistedColumnPrefs{VisibleExtras: []string{"col"}})

	assertWritesOnlyUnlocked(t, columnPrefsFilePath(),
		func() {
			persistColumnPrefsEntry("ctx\x00Later", persistedColumnPrefs{VisibleExtras: []string{"col"}})
		},
		func() bool {
			_, ok := loadColumnPrefsState().Contexts["ctx"]["Later"]
			return ok
		})
}

func TestPersistForgottenColumnPrefs_DeletesInsideTheLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	persistColumnPrefsEntry("ctx\x00Seed", persistedColumnPrefs{VisibleExtras: []string{"col"}})
	persistColumnPrefsEntry("ctx\x00Doomed", persistedColumnPrefs{VisibleExtras: []string{"col"}})

	assertWritesOnlyUnlocked(t, columnPrefsFilePath(),
		func() { persistForgottenColumnPrefs("ctx\x00Doomed") },
		func() bool {
			_, ok := loadColumnPrefsState().Contexts["ctx"]["Doomed"]
			return !ok
		})
}

func TestPersistRememberedSort_WritesInsideTheLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	persistRememberedSort("ctx\x00v1/seed", sortPref{column: "NAME", ascending: true})

	assertWritesOnlyUnlocked(t, sortMemoryFilePath(),
		func() { persistRememberedSort("ctx\x00v1/later", sortPref{column: "AGE", ascending: false}) },
		func() bool {
			_, ok := loadSortMemory()["ctx\x00v1/later"]
			return ok
		})
}

func TestPersistForgottenSort_DeletesInsideTheLock(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	persistRememberedSort("ctx\x00v1/seed", sortPref{column: "NAME", ascending: true})
	persistRememberedSort("ctx\x00v1/doomed", sortPref{column: "AGE", ascending: false})

	assertWritesOnlyUnlocked(t, sortMemoryFilePath(),
		func() { persistForgottenSort("ctx\x00v1/doomed") },
		func() bool {
			_, ok := loadSortMemory()["ctx\x00v1/doomed"]
			return !ok
		})
}
