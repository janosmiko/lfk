// Package app — atomic_write.go
// Durable atomic file write shared by the per-host caches (security
// availability, security findings). Writes to a unique temp file, fsyncs the
// file and its parent directory, then renames — so a crash mid-write can never
// leave a half-written file the loaders would have to defend against, and two
// concurrent writers can never collide on the temp path.
package app

import (
	"os"
	"path/filepath"
)

// writeFileDurable atomically writes data to path with owner-only (0600)
// permissions: write a unique temp file, fsync, chmod, rename, fsync parent
// dir. The caller is responsible for creating the parent directory. The temp
// file is removed on any failure before the rename.
//
// A unique temp name (os.CreateTemp, not a fixed "path.tmp" sibling) is
// required because these caches live in the shared kubectl cache dir: two lfk
// instances saving the same host file would otherwise truncate each other's
// in-flight temp write and promote a torn file via rename (see the same fix in
// cluster_colors.go).
func writeFileDurable(path string, data []byte) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	cleanup := func() { _ = os.Remove(tmp) }
	// CreateTemp already makes the file 0600; the explicit Chmod pins it so a
	// permissive umask or a pre-existing file at path cannot widen the result.
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	// Explicit Sync before rename: os.WriteFile closes without fsync, leaving a
	// window where the rename survives a crash but the contents do not.
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return err
	}
	if err := f.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		cleanup()
		return err
	}
	// Fsync the parent dir so the rename itself is durable on crash. A failure
	// here means the data is written but the dir entry may not survive a crash
	// on non-journaling filesystems — the file is still usable, so this is a
	// durability gap, not data loss.
	dirFd, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := dirFd.Sync()
	closeErr := dirFd.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
