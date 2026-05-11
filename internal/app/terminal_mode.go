package app

import (
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/ui"
)

// nextTerminalMode computes the next mode in the Ctrl+T cycle. Split out
// from the handler so the rotation logic can be unit-tested without
// stubbing OS lookups: hasMux is whether a tmux/zellij multiplexer is
// currently detected; isWindows is runtime.GOOS == "windows".
//
// On non-Windows hosts the cycle is pty -> exec -> mux -> pty when a
// multiplexer is available, and pty -> exec -> pty otherwise — Ctrl+T
// never lands the user in mux when it would just error on the next
// interactive shell.
//
// On Windows pty is excluded entirely (creack/pty has no Windows
// backend, so it would just trap the user in a broken state). The
// cycle is exec -> mux -> exec when a multiplexer is available, and
// exec -> exec otherwise — the Ctrl+T status message still updates,
// but the mode never silently transitions into something that fails
// on the next interactive action.
func nextTerminalMode(current string, hasMux, isWindows bool) string {
	if isWindows {
		switch current {
		case ui.TerminalModeExec:
			if hasMux {
				return ui.TerminalModeMux
			}
			return ui.TerminalModeExec
		case ui.TerminalModeMux:
			return ui.TerminalModeExec
		default:
			// Includes a stale pty value (e.g. left over before this
			// guard existed) and any unrecognised mode.
			return ui.TerminalModeExec
		}
	}
	switch current {
	case ui.TerminalModePTY:
		return ui.TerminalModeExec
	case ui.TerminalModeExec:
		if hasMux {
			return ui.TerminalModeMux
		}
		return ui.TerminalModePTY
	case ui.TerminalModeMux:
		return ui.TerminalModePTY
	default:
		return ui.TerminalModePTY
	}
}

func (m Model) handleExplorerActionKeyTerminalToggle() (tea.Model, tea.Cmd, bool) {
	mx := detectMultiplexer(nil, nil)
	ui.ConfigTerminalMode = nextTerminalMode(ui.ConfigTerminalMode, mx != nil, runtime.GOOS == "windows")

	switch ui.ConfigTerminalMode {
	case ui.TerminalModeExec:
		m.setStatusMessage("Terminal mode: exec (takes over the terminal)", false)
	case ui.TerminalModeMux:
		// nextTerminalMode only returns mux when hasMux was true, so mx
		// is non-nil here. Guard anyway so a future refactor that
		// bypasses nextTerminalMode can't trip a nil deref.
		muxName := "multiplexer"
		if mx != nil {
			muxName = mx.name
		}
		m.setStatusMessage("Terminal mode: mux (new "+muxName+" window or pane)", false)
	default:
		m.setStatusMessage("Terminal mode: pty (embedded in lfk)", false)
	}
	return m, scheduleStatusClear(), true
}
