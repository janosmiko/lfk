package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"sigs.k8s.io/yaml"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

// NamedSession is a whole-workspace snapshot saved under a user name. State is
// the same payload the auto-session persists; Name/SavedAt are metadata.
type NamedSession struct {
	Name    string       `json:"name" yaml:"name"`
	SavedAt time.Time    `json:"saved_at" yaml:"saved_at"`
	State   SessionState `json:"state" yaml:"state"`
}

const maxSessionNameLen = 64

// sanitizeSessionName reduces a display name to a safe filename stem: keep
// [a-zA-Z0-9], turn any run of other characters into a single '-', trim '-',
// and cap the length. Returns "" when nothing usable remains.
func sanitizeSessionName(name string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range strings.TrimSpace(name) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if len(out) > maxSessionNameLen {
		out = strings.Trim(out[:maxSessionNameLen], "-")
	}
	return out
}

func namedSessionsDir() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "sessions")
}

func namedSessionPath(name string) string {
	stem := sanitizeSessionName(name)
	dir := namedSessionsDir()
	if stem == "" || dir == "" {
		return ""
	}
	return filepath.Join(dir, stem+".yaml")
}

// saveNamedSession writes ns to <StateDir>/sessions/<sanitized>.yaml.
func saveNamedSession(ns NamedSession) error {
	path := namedSessionPath(ns.Name)
	if path == "" {
		return fmt.Errorf("invalid session name: %q", ns.Name)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := yaml.Marshal(ns)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// loadNamedSession reads a single named session by name. Returns an error if
// the file is missing or corrupt.
func loadNamedSession(name string) (*NamedSession, error) {
	path := namedSessionPath(name)
	if path == "" {
		return nil, fmt.Errorf("invalid session name: %q", name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var ns NamedSession
	if err := yaml.Unmarshal(data, &ns); err != nil {
		return nil, fmt.Errorf("corrupt session file %s: %w", path, err)
	}
	return &ns, nil
}

// listNamedSessions returns all parseable named sessions, newest first.
// Corrupt files are skipped with a warning.
func listNamedSessions() []NamedSession {
	dir := namedSessionsDir()
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []NamedSession
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		var ns NamedSession
		if err := yaml.Unmarshal(data, &ns); err != nil {
			logger.Warn("Skipping corrupt named session", "file", e.Name(), "error", err)
			continue
		}
		out = append(out, ns)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].SavedAt.After(out[j].SavedAt) })
	return out
}

// deleteNamedSession removes a named session file. The bool reports whether the
// file existed.
func deleteNamedSession(name string) (bool, error) {
	path := namedSessionPath(name)
	if path == "" {
		return false, fmt.Errorf("invalid session name: %q", name)
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// formatSavedAgo renders a compact relative age for the picker overlay.
func formatSavedAgo(saved, now time.Time) string {
	d := now.Sub(saved)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours())/24)
	}
}
