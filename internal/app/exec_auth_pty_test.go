//go:build !windows

package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

func withTerminalMode(t *testing.T, mode string) {
	t.Helper()
	old := ui.ConfigTerminalMode
	t.Cleanup(func() { ui.ConfigTerminalMode = old })
	ui.ConfigTerminalMode = mode
}

// startLoginPTY opens the login box for script, confirms it and feeds the
// PTY start back, as the program would.
func startLoginPTY(t *testing.T, script string) Model {
	t.Helper()
	withTerminalMode(t, ui.TerminalModePTY)
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")
	m.execAuth.cmd = loginCommand{argv: []string{"sh", "-c", script}, display: script}

	got, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	msg := cmd()
	require.IsType(t, execPTYStartMsg{}, msg)
	got, _ = got.(Model).Update(msg)
	m = got.(Model)
	require.Equal(t, modeExec, m.mode)
	t.Cleanup(m.cleanupExecPTY)
	return m
}

func waitLoginExit(t *testing.T, m Model) Model {
	t.Helper()
	done := m.execDone
	require.Eventually(t, done.Load, 5*time.Second, 10*time.Millisecond)
	require.Nil(t, m.execTickGen, "gen 0 skips the tick generation check")
	got, _ := m.Update(execPTYTickMsg{ptmx: m.execPTY})
	return got.(Model)
}

func TestExecAuthPTY_SuccessClosesAndRefreshes(t *testing.T) {
	m := waitLoginExit(t, startLoginPTY(t, "exit 0"))

	assert.Equal(t, modeExplorer, m.mode)
	assert.Nil(t, m.execPTY)
	assert.Contains(t, m.statusMessage, "Authenticated to test-ctx")
	assert.Empty(t, m.execAuth.context)
	assert.Nil(t, m.execAuth.ptyCmd)
}

func TestExecAuthPTY_FailureWaitsForKeyThenRetryBox(t *testing.T) {
	m := waitLoginExit(t, startLoginPTY(t, "exit 1"))

	require.Equal(t, modeExec, m.mode, "the user reads the error first")
	assert.Contains(t, stripANSI(m.View().Content), "Process exited")

	got, _ := m.Update(keyMsg("x"))
	m = got.(Model)
	assert.Equal(t, modeExplorer, m.mode)
	assert.Nil(t, m.execPTY)
	require.Equal(t, overlayConfirm, m.overlay)
	assert.Contains(t, loginBoxText(m), "attempt 2/3")
	assert.Nil(t, m.execAuth.ptyCmd)
	assert.Nil(t, m.execAuth.ptmx)
}

func TestExecAuthPTY_LeaveEarlyCancels(t *testing.T) {
	m := startLoginPTY(t, "sleep 5")
	done := m.execDone

	got, _ := m.Update(keyMsg("ctrl+]"))
	got, _ = got.(Model).Update(keyMsg("ctrl+]"))
	m = got.(Model)

	assert.Equal(t, modeExplorer, m.mode)
	assert.Nil(t, m.execPTY)
	assert.Empty(t, m.execAuth.context)
	assert.True(t, m.execAuth.tried["test-ctx"])
	assert.Contains(t, m.statusMessage, "Login to test-ctx skipped")
	require.Eventually(t, done.Load, 3*time.Second, 10*time.Millisecond, "the login must be killed")
	assert.NotEqual(t, overlayConfirm, m.overlay)
}

func TestExecAuthPTY_TabSwitchCancels(t *testing.T) {
	m := startLoginPTY(t, "sleep 5")
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0

	got, _ := m.Update(keyMsg("ctrl+]"))
	got, _ = got.(Model).Update(keyMsg("]"))
	m = got.(Model)

	assert.Equal(t, 1, m.activeTab)
	assert.Empty(t, m.execAuth.context)
	assert.Nil(t, m.execAuth.ptmx)
	assert.Nil(t, m.tabs[0].execPTY, "the login must not stay in a background tab")
}

func TestExecAuthPTY_TabClickCancels(t *testing.T) {
	m := startLoginPTY(t, "sleep 5")
	m.tabs = []TabState{{}, {}}
	m.activeTab = 0
	x := -1
	for i := 0; i < m.width && x < 0; i++ {
		if m.tabAtX(i) == 1 {
			x = i
		}
	}
	require.GreaterOrEqual(t, x, 0, "tab 2 must be on the tab bar")

	got, _ := m.Update(tea.MouseClickMsg{Button: tea.MouseLeft, X: x, Y: 1})
	m = got.(Model)

	assert.Equal(t, 1, m.activeTab)
	assert.Empty(t, m.execAuth.context)
	assert.Nil(t, m.execAuth.ptmx)
	assert.Nil(t, m.tabs[0].execPTY, "the login must not stay in a background tab")
}

func TestExecAuthPTY_StartFailureOpensRetryBox(t *testing.T) {
	withTerminalMode(t, ui.TerminalModePTY)
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")
	m.execAuth.cmd = loginCommand{argv: []string{"/nonexistent/lfk-login"}, display: "x"}

	got, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd()
	require.IsType(t, execAuthDoneMsg{}, msg)
	got, _ = got.(Model).Update(msg)
	m = got.(Model)
	require.Equal(t, overlayConfirm, m.overlay)
	assert.Contains(t, loginBoxText(m), "attempt 2/3")
}

func TestExecAuthPTY_ExecModeUsesTeaExec(t *testing.T) {
	withTerminalMode(t, ui.TerminalModeExec)
	m := basePush80Model()
	m.nav.Level = model.LevelResourceTypes
	m = openLoginBox(t, m, "test-ctx")

	got, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.NotNil(t, cmd)
	_, isPTY := cmd().(execPTYStartMsg)
	assert.False(t, isPTY)
	assert.Nil(t, got.(Model).execAuth.ptyCmd)
}
