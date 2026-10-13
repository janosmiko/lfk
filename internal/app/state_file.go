package app

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// stateFilePath returns "" when the state dir can't be resolved, so callers
// degrade to a best-effort no-op instead of failing.
func stateFilePath(name string) string {
	path, err := stateFilePathErr(name)
	if err != nil {
		return ""
	}
	return path
}

// stateFilePathErr is for write paths: a preference that cannot be saved must
// tell the user why.
func stateFilePathErr(name string) (string, error) {
	dir, err := paths.StateDir()
	if err != nil {
		return "", fmt.Errorf("no state directory: %w", err)
	}
	return filepath.Join(dir, name), nil
}

// loadStateFileStrict is the loader for read-modify-write paths. A missing
// file is empty state. Any other read error or a decode error is returned, so
// the caller never saves over a file it could not read.
func loadStateFileStrict[T any](name string) (T, error) {
	var zero T
	path, err := stateFilePathErr(name)
	if err != nil {
		return zero, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return zero, nil
	}
	if err != nil {
		return zero, fmt.Errorf("%s is unreadable: %w", path, err)
	}
	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		return zero, fmt.Errorf("%s is corrupt, fix or remove it: %w", path, err)
	}
	return v, nil
}

// saveStateFileGuarded is saveStateFile for user-action saves of a whole-state
// file. Under the file lock it first checks that any existing file reads and
// decodes, and returns an error without chmod or write when it does not.
func saveStateFileGuarded[T any](name string, v T) error {
	path, err := stateFilePathErr(name)
	if err != nil {
		return err
	}
	return updateStateFile(path, func() error {
		if _, err := loadStateFileStrict[T](name); err != nil {
			return err
		}
		return saveStateFile(name, v)
	})
}

// reportPrefSaveErr shows a failed preference save in the status bar. It
// returns nil when err is nil.
func (m *Model) reportPrefSaveErr(err error) tea.Cmd {
	if err == nil {
		return nil
	}
	m.setStatusMessage("Could not save preference: "+err.Error(), true)
	return scheduleStatusClear()
}

// loadStateFile returns the zero value on any failure: a hand-edited or
// absent state file must never stop the app from starting.
func loadStateFile[T any](name string) T {
	var zero T
	path := stateFilePath(name)
	if path == "" {
		return zero
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Warn("Failed to read state file", "error", err, "path", path)
		}
		return zero
	}
	var v T
	if err := yaml.Unmarshal(data, &v); err != nil {
		logger.Warn("State file is corrupt; ignoring", "error", err, "path", path)
		return zero
	}
	return v
}

// saveStateFile renames a temp file into place but skips fsync: several callers run inline on
// every keypress, and File.Sync costs tens of ms on macOS (F_FULLFSYNC). A
// caller that needs durability calls writeFileDurable(stateFilePath(name), data).
func saveStateFile[T any](name string, v T) error {
	path, err := stateFilePathErr(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return writeFileReplace(path, data, false)
}
