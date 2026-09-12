package app

import (
	"os"
	"path/filepath"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// stateFilePath returns "" when the state dir can't be resolved, so callers
// degrade to a best-effort no-op instead of failing.
func stateFilePath(name string) string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, name)
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

// saveStateFile is a no-op (nil error) when the state dir can't be resolved,
// matching the existing best-effort contract of the per-site save funcs.
func saveStateFile[T any](name string, v T) error {
	path := stateFilePath(name)
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return writeFileDurable(path, data)
}
