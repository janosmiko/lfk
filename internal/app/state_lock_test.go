package app

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// TestPersistViewerPref_ConcurrentWritersKeepEachOthersToggles is the case the
// lock exists for. Each writer opens the lock file separately, the way two lfk
// processes would. Without the lock the last writer wins and the others'
// toggles vanish.
//
// The assertion covers the writers that got the lock, not all of them. Each
// write loads, merges and fsyncs, so on a slow machine a writer can wait out
// stateLockBudget and skip, which withStateFileLock documents as the right
// move: better to drop this write than clobber the other process.
func TestPersistViewerPref_ConcurrentWritersKeepEachOthersToggles(t *testing.T) {
	isolateViewerPrefs(t)

	wrote := make([]bool, numViewerPrefs)
	var wg sync.WaitGroup
	for i := range int(numViewerPrefs) {
		wg.Go(func() { wrote[i] = persistViewerPref(viewerPref(i), true) })
	}
	wg.Wait()

	got := 0
	for _, ok := range wrote {
		if ok {
			got++
		}
	}
	if got < 2 {
		t.Fatalf("%d of %d writers got the lock, too few to prove a merge", got, numViewerPrefs)
	}

	s := loadViewerPrefsState()
	for i, b := range viewerPrefBindings {
		if !wrote[i] {
			continue // this writer waited out the budget and skipped
		}
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

// lockHelperEnv carries the lock-file path to the child half of
// TestWithStateFileLock_ContendsAcrossProcesses.
const lockHelperEnv = "LFK_TEST_HOLD_LOCK_PATH"

// TestHoldStateLockHelper is the child process, not a test of its own. The
// parent re-execs the test binary with lockHelperEnv set. It takes the lock,
// announces it on stdout, and holds it until the parent closes stdin.
func TestHoldStateLockHelper(t *testing.T) {
	path := os.Getenv(lockHelperEnv)
	if path == "" {
		t.Skip("child-process helper, driven by TestWithStateFileLock_ContendsAcrossProcesses")
	}
	lock := flock.New(path)
	if err := lock.Lock(); err != nil {
		t.Fatalf("child could not take the lock: %v", err)
	}
	defer func() { _ = lock.Unlock() }()

	fmt.Println("LOCKED")
	// Blocks until the parent closes our stdin.
	_, _ = io.Copy(io.Discard, os.Stdin)
}

// TestWithStateFileLock_ContendsAcrossProcesses proves the property the lock
// exists for. The same-process tests contend on two file descriptors, which is
// what flock(2) and LockFileEx key on, but only a second process shows that two
// lfk instances actually exclude each other.
func TestWithStateFileLock_ContendsAcrossProcesses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "thing.yaml")
	lockPath := filepath.Join(dir, ".thing.yaml.lock")

	child := exec.Command(os.Args[0], "-test.run=^TestHoldStateLockHelper$", "-test.timeout=60s")
	child.Env = append(os.Environ(), lockHelperEnv+"="+lockPath)
	stdin, err := child.StdinPipe()
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	stdout, err := child.StdoutPipe()
	if err != nil {
		t.Fatalf("test setup: %v", err)
	}
	if err := child.Start(); err != nil {
		t.Fatalf("test setup: %v", err)
	}
	t.Cleanup(func() {
		_ = stdin.Close()
		_ = child.Wait()
	})

	if !awaitLine(t, stdout, "LOCKED") {
		t.Fatal("the child never reported that it holds the lock")
	}

	if withStateFileLock(path, func() {}) {
		t.Error("the lock was granted while another process holds it")
	}

	// Releasing the child's stdin ends the child, which drops the lock.
	_ = stdin.Close()
	if err := child.Wait(); err != nil {
		t.Fatalf("child exited badly: %v", err)
	}

	if !withStateFileLock(path, func() {}) {
		t.Error("the lock was refused after the holding process exited")
	}
}

// awaitLine reports whether want shows up on r before the test deadline.
func awaitLine(t *testing.T, r io.Reader, want string) bool {
	t.Helper()
	found := make(chan bool, 1)
	go func() {
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if strings.TrimSpace(sc.Text()) == want {
				found <- true
				return
			}
		}
		found <- false
	}()
	select {
	case ok := <-found:
		return ok
	case <-time.After(30 * time.Second):
		return false
	}
}
