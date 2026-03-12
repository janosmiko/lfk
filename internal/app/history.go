package app

import (
	"os"
	"path/filepath"
	"strings"
)

const maxHistoryEntries = 500

// commandHistory manages persistent shell command history for the command bar.
type commandHistory struct {
	entries []string
	cursor  int    // -1 means "not browsing history" (typing new command)
	draft   string // saves what the user was typing before browsing history
}

// historyFilePath returns the path to the command history file.
// Uses $XDG_STATE_HOME/lfk/history (defaults to ~/.local/state/lfk/history).
func historyFilePath() string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", "history")
}

// loadCommandHistory reads command history from disk and returns a new commandHistory.
func loadCommandHistory() *commandHistory {
	h := &commandHistory{cursor: -1}
	path := historyFilePath()
	if path == "" {
		return h
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return h
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for _, line := range lines {
		if line != "" {
			h.entries = append(h.entries, line)
		}
	}
	// Keep only the most recent entries if the file exceeds the limit.
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[len(h.entries)-maxHistoryEntries:]
	}
	return h
}

// add appends a command to the history. Empty commands and consecutive
// duplicates are ignored.
func (h *commandHistory) add(cmd string) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return
	}
	// Deduplicate consecutive entries.
	if len(h.entries) > 0 && h.entries[len(h.entries)-1] == cmd {
		return
	}
	h.entries = append(h.entries, cmd)
	// Trim oldest entries if we exceed the limit.
	if len(h.entries) > maxHistoryEntries {
		h.entries = h.entries[len(h.entries)-maxHistoryEntries:]
	}
}

// save writes the current history entries to disk.
func (h *commandHistory) save() {
	path := historyFilePath()
	if path == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	content := strings.Join(h.entries, "\n") + "\n"
	_ = os.WriteFile(path, []byte(content), 0o644)
}

// up navigates to the previous (older) history entry.
// On the first call, it saves the current input as a draft.
// Returns the history entry to display in the command bar.
func (h *commandHistory) up(currentInput string) string {
	if len(h.entries) == 0 {
		return currentInput
	}
	if h.cursor == -1 {
		// First time pressing up: save the current input as draft.
		h.draft = currentInput
		h.cursor = len(h.entries) - 1
	} else if h.cursor > 0 {
		h.cursor--
	}
	return h.entries[h.cursor]
}

// down navigates to the next (newer) history entry.
// If at the end of history, restores the saved draft.
// Returns the text to display in the command bar.
func (h *commandHistory) down() string {
	if h.cursor == -1 {
		return h.draft
	}
	h.cursor++
	if h.cursor >= len(h.entries) {
		h.cursor = -1
		return h.draft
	}
	return h.entries[h.cursor]
}

// reset resets the cursor position, clearing any history browsing state.
func (h *commandHistory) reset() {
	h.cursor = -1
	h.draft = ""
}
