package ui

import (
	"runtime"
	"strings"

	"github.com/janosmiko/lfk/internal/logger"
)

// applyTerminalConfig wires the `terminal` YAML key into ConfigTerminalMode.
func applyTerminalConfig(terminal string) {
	if terminal == "" {
		return
	}
	mode, warning := resolveTerminalMode(terminal, runtime.GOOS, ConfigTerminalMode)
	if warning != "" {
		// The raw value stays out of the log per redaction policy. The
		// user's own config file records what they typed.
		logger.Warn(warning,
			"valid", []string{TerminalModePTY, TerminalModeExec, TerminalModeMux},
			"applied", mode)
	}
	ConfigTerminalMode = mode
}

// DefaultExecShells is the shell preference order for exec into a Linux
// container. The last entry runs unconditionally, so it must be the shell that
// every image ships.
var DefaultExecShells = []string{"bash -i", "ash", "sh"}

// ConfigExecShells is the shell preference order exec uses, set from the
// `exec_shells` YAML key.
var ConfigExecShells = DefaultExecShells

// execShellEntryAllowed rejects anything but a plain command line. lfk pastes
// each entry into a shell command that runs inside the container, so a
// separator, a redirect, or a substitution would run as a second command.
func execShellEntryAllowed(entry string) bool {
	if entry == "" {
		return false
	}
	for _, r := range entry {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case strings.ContainsRune(" ._/+=:-", r):
		default:
			return false
		}
	}
	return true
}

// applyExecShellsConfig wires the `exec_shells` YAML key into ConfigExecShells.
// An empty or fully rejected list keeps the defaults, because an empty order
// would leave exec with no shell to start.
func applyExecShellsConfig(shells []string) {
	kept := make([]string, 0, len(shells))
	for _, raw := range shells {
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}
		if !execShellEntryAllowed(entry) {
			// The rejected value stays out of the log per redaction
			// policy. A rejected entry can hold anything, a pasted
			// secret included.
			logger.Warn("Ignoring an exec_shells entry: only letters, digits, spaces, and ._/+=:- are allowed")
			continue
		}
		kept = append(kept, entry)
	}
	if len(kept) == 0 {
		ConfigExecShells = DefaultExecShells
		return
	}
	ConfigExecShells = kept
}
