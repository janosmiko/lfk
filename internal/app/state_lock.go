package app

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"

	"github.com/janosmiko/lfk/internal/logger"
)

// stateLockBudget caps how long a load-modify-save waits for the lock. The
// critical section is a few milliseconds, so this is only reached under real
// contention, and it runs on the Bubble Tea Update goroutine where a longer
// wait would stall the frame.
const stateLockBudget = 250 * time.Millisecond

// stateLockRetry is how often the waiter re-tries while inside the budget.
const stateLockRetry = 5 * time.Millisecond

// withStateFileLock runs fn while holding an exclusive interprocess lock for
// one state file, so a read-modify-write cannot lose a concurrent lfk
// instance's change. It reports whether fn ran: on a timeout the caller skips
// the write rather than clobbering the other process.
//
// The lock lives on a sibling ".lock" file, never on the state file itself.
// writeFileDurable renames a temp file over the target, which replaces the
// inode a lock on the target would be attached to.
func withStateFileLock(path string, fn func()) bool {
	dir := filepath.Dir(path)
	// The lock file needs the state directory to exist. The writer inside fn
	// creates it too, but that runs after the lock is already taken.
	if err := os.MkdirAll(dir, 0o700); err != nil {
		logger.Warn("Could not create the state directory; skipping this write",
			"error", err, "path", dir)
		return false
	}
	lockPath := filepath.Join(dir, "."+filepath.Base(path)+".lock")
	lock := flock.New(lockPath)

	ctx, cancel := context.WithTimeout(context.Background(), stateLockBudget)
	defer cancel()

	locked, err := lock.TryLockContext(ctx, stateLockRetry)
	if err != nil || !locked {
		logger.Warn("Could not lock state file; skipping this write",
			"error", err, "path", path)
		return false
	}
	defer func() {
		if err := lock.Unlock(); err != nil {
			logger.Warn("Failed to release state file lock", "error", err, "path", lockPath)
		}
	}()

	fn()
	return true
}
