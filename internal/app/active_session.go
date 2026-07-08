package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// The "active session" is the named session that auto-save writes to and that
// startup restores. An empty name means the built-in default workspace, which
// is stored in the legacy session.yaml (see sessionFilePath) — there is no
// sessions/default.yaml file. The active name is persisted so quitting on a
// named session and relaunching reopens it.

// activeSessionFilePath returns the path to the file recording the active
// session name. Empty string when the state dir cannot be resolved.
func activeSessionFilePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "active_session")
}

// loadActiveSessionName reads the persisted active session name. Returns "" for
// the default workspace when the file is absent, empty, or unreadable.
func loadActiveSessionName() string {
	path := activeSessionFilePath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// saveActiveSessionName persists the active session name. An empty name (the
// default workspace) removes the pointer file so a fresh install and an
// explicit "back to default" both resolve to the default.
func saveActiveSessionName(name string) error {
	path := activeSessionFilePath()
	if path == "" {
		return nil
	}
	name = strings.TrimSpace(name)
	if name == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(name+"\n"), 0o600)
}

// resolveStartupSession picks the session name to open at startup. Precedence:
// the --session flag / LFK_SESSION env value (cliSession), then the persisted
// active session, then "" (default). A non-default result is persisted so it
// sticks across restarts.
func resolveStartupSession(cliSession string) string {
	name := strings.TrimSpace(cliSession)
	if name == "" {
		return loadActiveSessionName()
	}
	if err := saveActiveSessionName(name); err != nil {
		logger.Warn("Failed to persist active session from --session", "session", name, "error", err)
	}
	return name
}
