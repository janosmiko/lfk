package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/model"
)

// bookmarksFilePath returns the path to the bookmarks file.
// Uses $XDG_STATE_HOME/lfk/ (defaults to ~/.local/state/lfk/) per XDG specification.
func bookmarksFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "bookmarks.yaml")
}

// loadBookmarks reads bookmarks from the YAML file on disk.
// Falls back to the legacy ~/.config/lfk/ location and migrates if found.
// If the primary file is missing or corrupt, tries the backup file.
func loadBookmarks() []model.Bookmark {
	path := bookmarksFilePath()
	if path == "" {
		return nil
	}
	if bm := loadBookmarksFromFile(path); bm != nil {
		return bm
	}
	// Try legacy location and migrate.
	data := migrateStateFile("bookmarks.yaml", path)
	if data != nil {
		var bookmarks []model.Bookmark
		if err := yaml.Unmarshal(data, &bookmarks); err == nil && len(bookmarks) > 0 {
			return bookmarks
		}
	}
	// Try the backup file as a last resort.
	bakPath := path + ".bak"
	if bm := loadBookmarksFromFile(bakPath); bm != nil {
		logger.Info("Loaded bookmarks from backup file", "path", bakPath)
		// Restore the primary file from backup.
		if err := saveBookmarks(bm); err != nil {
			logger.Error("Failed to restore bookmarks from backup", "error", err, "path", path)
		}
		return bm
	}
	return nil
}

// loadBookmarksFromFile reads and parses bookmarks from a specific file path.
func loadBookmarksFromFile(path string) []model.Bookmark {
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read bookmarks", "error", err, "path", path)
		}
		return nil
	}
	if len(data) == 0 {
		return nil
	}
	var bookmarks []model.Bookmark
	if err := yaml.Unmarshal(data, &bookmarks); err != nil {
		logger.Warn("Bookmarks file is corrupt; trying backup", "error", err, "path", path)
		return nil
	}
	return bookmarks
}

// saveBookmarks writes bookmarks to the YAML file on disk using an atomic
// write (write to temp file, fsync, then rename) to prevent data loss if the
// process is interrupted mid-write. A backup of the previous file is kept.
func saveBookmarks(bookmarks []model.Bookmark) error {
	path := bookmarksFilePath()
	if path == "" {
		return nil
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(bookmarks)
	if err != nil {
		return err
	}
	bakPath := path + ".bak"
	if len(bookmarks) == 0 {
		// Explicit delete-all: remove the backup so it doesn't resurrect.
		_ = os.Remove(bakPath)
	} else if info, statErr := os.Stat(path); statErr == nil && info.Size() > 0 {
		// Keep a backup of the current file before overwriting.
		_ = copyFile(path, bakPath)
	}
	// Atomic write: write to a temp file in the same directory, fsync, then rename.
	tmp, err := os.CreateTemp(dir, ".bookmarks-*.yaml.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	// Fsync to ensure data is flushed to stable storage before rename.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}
	return os.Rename(tmpPath, path)
}

// copyFile copies src to dst, overwriting dst if it exists.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

// removeBookmark removes the bookmark at the given index.
// Returns a new slice; the original is never mutated.
func removeBookmark(bookmarks []model.Bookmark, idx int) []model.Bookmark {
	if idx < 0 || idx >= len(bookmarks) {
		return bookmarks
	}
	result := make([]model.Bookmark, 0, len(bookmarks)-1)
	result = append(result, bookmarks[:idx]...)
	result = append(result, bookmarks[idx+1:]...)
	return result
}
