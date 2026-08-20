package app

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/paths"
)

const maxHistoryEntries = 500

// Filenames for the persistent input histories in the lfk state directory.
// `/` (search) and `f` (filter) share one file: the query syntax and matched
// fields are identical between the two, only the action on a match differs
// (jump vs. narrow), so users want to recall the same query regardless of
// which mode they re-enter. The `:` command bar stays separate because its
// inputs are kubectl-shaped commands, not resource-name queries. The log
// viewer's `/` search is also kept separate: it matches against raw log
// lines (substring/regex over arbitrary text), not resource names, so
// pooling it with the explorer query history would surface irrelevant
// entries on Up/Down in either context.
const (
	historyFileCommand   = "history"
	historyFileQuery     = "query-history"
	historyFileLogSearch = "log-search-history"
)

// commandHistory manages a persistent ring of recent text-input entries.
// Used by the command bar, the explorer search and filter inputs, and
// the log viewer's `/` search (per filename).
type commandHistory struct {
	entries  []string
	cursor   int    // -1 means "not browsing history" (typing new command)
	draft    string // saves what the user was typing before browsing history
	filename string // file in the lfk state directory; empty = command bar (back-compat)
}

// historyFilePathFor returns the path to the named history file. Returns
// "" when the state directory cannot be resolved.
func historyFilePathFor(name string) string {
	dir, err := paths.StateDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, name)
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
	// 0700/0600: history files persist raw search/filter/log-search
	// queries that may contain sensitive fragments (tokens, emails),
	// so restrict them to the owning user — same convention as
	// ~/.bash_history etc.
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		logger.Warn("Failed to create history directory", "error", err, "path", path)
		return
	}
	content := strings.Join(h.entries, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		logger.Warn("Failed to write history file", "error", err, "path", path)
	}
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

// leaveBrowse exits history browsing while preserving the pre-recall
// draft. Use this when the user edits a recalled entry (typing,
// backspace, ctrl+w, ctrl+u): the edited text becomes the new content,
// but a subsequent Down past the newest entry should restore the
// original draft the user had before pressing Up — not "" (which is
// what reset() would leave behind). Nil receiver no-op.
func (h *commandHistory) leaveBrowse() {
	if h == nil {
		return
	}
	h.cursor = -1
}
