package app

import (
	"errors"
	"os/exec"
	"runtime"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/ui"
)

func execAuthUsesPTY() bool {
	return ui.ConfigTerminalMode == ui.TerminalModePTY && runtime.GOOS != "windows"
}

// startExecAuthPTY runs the login in the embedded terminal. The pty owns the
// login's stdin, stdout and stderr, because the cmd leaves them nil.
func (m Model) startExecAuthPTY(c *execAuthCmd) (tea.Model, tea.Cmd) {
	ctx := c.context
	cmd, err := c.newLoginCmd()
	if err != nil {
		m.setErrorFromErr("Auth failed: ", err)
		return m.updateExecAuthDone(execAuthDoneMsg{context: ctx, err: err})
	}
	m.execAuth.ptyCmd = cmd
	cols, rows := m.embeddedPTYSize()
	start := startPTYExecCmd(cmd, fmtPTYTitle("login: "+ui.SanitizeTerminalText(ctx)), cols, rows)
	return m, func() tea.Msg {
		msg := start()
		// Without this the login stays pending and blocks every later prompt.
		if r, ok := msg.(actionResultMsg); ok {
			return execAuthDoneMsg{context: ctx, err: r.err}
		}
		return msg
	}
}

// execAuthPTYResult reports the login result once the focused login PTY exits.
// ProcessState is safe to read here, since the reader stores done after Wait.
func (m Model) execAuthPTYResult() (execAuthDoneMsg, bool) {
	if !m.execAuthPTYFocused() || m.execDone == nil || !m.execDone.Load() {
		return execAuthDoneMsg{}, false
	}
	msg := execAuthDoneMsg{context: m.execAuth.context}
	switch ps := m.execAuth.ptyCmd.ProcessState; {
	case ps == nil:
		msg.err = errors.New("login did not finish")
	case !ps.Success():
		msg.err = &exec.ExitError{ProcessState: ps}
	}
	return msg, true
}

func (m Model) execAuthPTYFocused() bool {
	return m.execAuth.ptmx != nil && m.execPTY == m.execAuth.ptmx
}

// leaveExecAuthPTY cancels the focused login before next runs. Once the user
// leaves the terminal, the login result has nowhere to go.
func (m Model) leaveExecAuthPTY(next func(Model) (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	if p := m.execAuth.ptyCmd.Process; p != nil {
		_ = p.Kill()
	}
	m.cleanupExecPTY()
	m.mode = modeExplorer
	mdl, cancel := m.cancelExecAuthLogin()
	mdl, cmd := next(mdl.(Model))
	return mdl, tea.Batch(cancel, cmd)
}
