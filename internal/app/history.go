package app

import (
	"os"
	"path/filepath"
	"strings"
)

const maxHistoryEntries = 500

// Filenames for the persistent input histories under $XDG_STATE_HOME/lfk/.
// `/` (search) and `f` (filter) share one file: the query syntax and matched
// fields are identical between the two, only the action on a match differs
// (jump vs. narrow), so users want to recall the same query regardless of
// which mode they re-enter. The `:` command bar stays separate because its
// inputs are kubectl-shaped commands, not resource-name queries.
const (
	historyFileCommand = "history"
	historyFileQuery   = "query-history"
)

// commandHistory manages a persistent ring of recent text-input entries.
// Used by the command bar and (per filename) the explorer search and
// filter inputs.
type commandHistory struct {
	entries  []string
	cursor   int    // -1 means "not browsing history" (typing new command)
	draft    string // saves what the user was typing before browsing history
	filename string // file under $XDG_STATE_HOME/lfk/; empty = command bar (back-compat)
}

// historyFilePath returns the path to the command bar history file.
// Uses $XDG_STATE_HOME/lfk/history (defaults to ~/.local/state/lfk/history).
func historyFilePath() string {
	return historyFilePathFor(historyFileCommand)
}

// historyFilePathFor returns the path to the named history file. Returns
// "" when the home directory cannot be resolved.
func historyFilePathFor(name string) string {
	stateDir := os.Getenv("XDG_STATE_HOME")
	if stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		stateDir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(stateDir, "lfk", name)
}

// loadCommandHistory reads command bar history from disk.
func loadCommandHistory() *commandHistory {
	return loadInputHistory(historyFileCommand)
}

// loadInputHistory reads the named history file from disk and returns a new
// commandHistory bound to that filename. The filename is used by save() so
// every loaded instance writes back to the file it came from.
func loadInputHistory(name string) *commandHistory {
	h := &commandHistory{cursor: -1, filename: name}
	path := historyFilePathFor(name)
	if path == "" {
		return h
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return h
	}
	lines := strings.SplitSeq(strings.TrimRight(string(data), "\n"), "\n")
	for line := range lines {
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
// duplicates are ignored. Nil receiver is a no-op so test models that
// don't initialize history don't panic.
func (h *commandHistory) add(cmd string) {
	if h == nil {
		return
	}
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

// save writes the current history entries to disk. Nil receiver no-op.
func (h *commandHistory) save() {
	if h == nil {
		return
	}
	name := h.filename
	if name == "" {
		name = historyFileCommand
	}
	path := historyFilePathFor(name)
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
// Nil receiver returns currentInput unchanged.
func (h *commandHistory) up(currentInput string) string {
	if h == nil || len(h.entries) == 0 {
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
// Nil receiver returns "".
func (h *commandHistory) down() string {
	if h == nil {
		return ""
	}
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
// Nil receiver no-op.
func (h *commandHistory) reset() {
	if h == nil {
		return
	}
	h.cursor = -1
	h.draft = ""
}
