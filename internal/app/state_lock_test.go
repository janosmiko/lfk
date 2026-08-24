package app

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gofrs/flock"
)

func TestWithStateFileLock_RunsAndReleases(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "thing.yaml")

	ran := 0
	for range 3 {
		if !withStateFileLock(path, func() { ran++ }) {
			t.Fatal("a lock nobody else holds must be granted")
		}
	}
	if ran != 3 {
		t.Errorf("fn ran %d times, want 3: the lock is not being released", ran)
	}
}

func TestWithStateFileLock_GivesUpWhenHeld(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thing.yaml")
	held := flock.New(filepath.Join(dir, ".thing.yaml.lock"))
	if err := held.Lock(); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	defer func() { _ = held.Unlock() }()

	start := time.Now()
	ran := false
	got := withStateFileLock(path, func() { ran = true })

	if got || ran {
		t.Error("a lock another holder owns must not be granted, and fn must not run")
	}
	if elapsed := time.Since(start); elapsed > 2*stateLockBudget {
		t.Errorf("waited %v, want the caller released near the %v budget", elapsed, stateLockBudget)
	}
}

// TestPersistViewerPref_ConcurrentTogglesAllSurvive is the case the lock exists
// for. Each writer opens the lock file separately, the way two lfk processes
// would. Without the lock the last writer wins and the others' toggles vanish.
func TestPersistViewerPref_ConcurrentTogglesAllSurvive(t *testing.T) {
	isolateViewerPrefs(t)

	var wg sync.WaitGroup
	for i := range int(numViewerPrefs) {
		wg.Go(func() { persistViewerPref(viewerPref(i), true) })
	}
	wg.Wait()

	s := loadViewerPrefsState()
	for i, b := range viewerPrefBindings {
		v := *b.field(&s)
		if v == nil {
			t.Errorf("pref %d was dropped: a concurrent writer overwrote it", i)
			continue
		}
		if !*v {
			t.Errorf("pref %d is false, want true", i)
		}
	}
}
