package app

import (
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/janosmiko/lfk/internal/paths"
)

// tipStatePath returns the path of the tip-rotation cursor file, or "" when
// no state directory is available (tips then fall back to a random pick).
func tipStatePath() string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, "tip-cursor")
}

// nextStartupTip returns the next tip in the rotation and advances the
// persisted cursor, so every tip is shown once before any repeats across
// restarts.
func nextStartupTip() string {
	return nextTipFromFile(tipStatePath(), startupTips)
}

// nextTipFromFile reads the rotation cursor from path, returns the tip it
// points at, and persists the incremented cursor. A missing or corrupt
// cursor starts the rotation at a random offset so fresh installs don't all
// begin with the same tip; a cursor beyond the list (the list shrank) wraps.
// State I/O is best-effort: on failure the tip is still returned.
func nextTipFromFile(path string, tips []string) string {
	if len(tips) == 0 {
		return ""
	}
	cursor := -1
	if path != "" {
		if data, err := os.ReadFile(path); err == nil {
			if n, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil && n >= 0 {
				cursor = n % len(tips)
			}
		}
	}
	if cursor < 0 {
		cursor = rand.IntN(len(tips))
	}
	if path != "" {
		next := (cursor + 1) % len(tips)
		_ = os.WriteFile(path, []byte(strconv.Itoa(next)), 0o600)
	}
	return tips[cursor]
}
