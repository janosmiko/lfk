package app

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/janosmiko/lfk/internal/ui"
)

func runBackground(t *testing.T, command string) any {
	t.Helper()
	return runBackgroundOn(t, testModelExec(), command)
}

func runBackgroundOn(t *testing.T, m Model, command string) any {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}
	cmd := m.execCustomAction(ui.CustomAction{Label: "Label it", Command: command, Background: true}, command)
	require.NotNil(t, cmd)
	return cmd()
}

func TestExecCustomAction_BackgroundSuccess(t *testing.T) {
	msg := runBackground(t, "echo noisy output")
	res, ok := msg.(actionResultMsg)
	require.True(t, ok, "got %T", msg)
	assert.NoError(t, res.err)
	assert.Equal(t, "Label it completed", res.message)
}

func TestExecCustomAction_ForegroundStillUsesExecProcess(t *testing.T) {
	m := testModelExec()
	cmd := m.execCustomAction(ui.CustomAction{Label: "x", Command: "true"}, "true")
	require.NotNil(t, cmd)
	_, isResult := cmd().(actionResultMsg)
	assert.False(t, isResult)
}

func TestExecCustomAction_BackgroundFailureWithOutput(t *testing.T) {
	msg := runBackground(t, "echo boom; exit 3")
	res, ok := msg.(customActionFailedMsg)
	require.True(t, ok, "got %T", msg)
	assert.Equal(t, "Label it failed (exit 3)", res.title)
	assert.Equal(t, "boom", res.output)
}

func TestExecCustomAction_BackgroundFailureNoOutput(t *testing.T) {
	msg := runBackground(t, "exit 2")
	res, ok := msg.(actionResultMsg)
	require.True(t, ok, "got %T", msg)
	assert.Error(t, res.err)
}

func TestExecCustomAction_BackgroundOutputCapped(t *testing.T) {
	msg := runBackground(t, "head -c 3000000 /dev/zero | tr '\\0' a; exit 1")
	res, ok := msg.(customActionFailedMsg)
	require.True(t, ok, "got %T", msg)
	body, found := strings.CutSuffix(res.output, "\n[output truncated]")
	assert.True(t, found, "missing truncation notice")
	assert.LessOrEqual(t, len(body), maxCustomActionOutput)
	assert.Equal(t, strings.Repeat("a", len(body)), body)
}

func TestCappedBuffer_WriteReportsFullLengthPastCap(t *testing.T) {
	var c cappedBuffer
	chunk := make([]byte, maxCustomActionOutput-10)
	n, err := c.Write(chunk)
	require.NoError(t, err)
	assert.Equal(t, len(chunk), n)
	assert.False(t, c.truncated)
	n, err = c.Write(make([]byte, 100))
	require.NoError(t, err)
	assert.Equal(t, 100, n)
	assert.Equal(t, maxCustomActionOutput, c.buf.Len())
	assert.True(t, c.truncated)
}

func TestExecCustomAction_BackgroundGrandchildDoesNotBlock(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}
	done := make(chan any, 1)
	go func() { done <- runBackground(t, "sleep 4 & echo started; exit 3") }()
	select {
	case msg := <-done:
		res, ok := msg.(customActionFailedMsg)
		require.True(t, ok, "got %T", msg)
		assert.Equal(t, "Label it failed (exit 3)", res.title)
	case <-time.After(10 * time.Second):
		t.Fatal("background action blocked on a held output pipe")
	}
}

func TestExecCustomAction_BackgroundSuccessWithHeldPipe(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}
	msg := runBackground(t, "sleep 4 & exit 0")
	res, ok := msg.(actionResultMsg)
	require.True(t, ok, "got %T", msg)
	assert.NoError(t, res.err)
	assert.Equal(t, "Label it completed", res.message)
}

func TestExecCustomAction_QuitCancelsRunningAction(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("requires sh")
	}
	ctx, cancel := context.WithCancel(t.Context())
	m := testModelExec()
	m.quitCtx = ctx
	pidFile := filepath.Join(t.TempDir(), "pid")
	done := make(chan any, 1)
	go func() { done <- runBackgroundOn(t, m, "echo $$ > "+pidFile+"; exec sleep 30") }()

	var pid int
	require.Eventually(t, func() bool {
		b, err := os.ReadFile(pidFile)
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(b)))
		return err == nil
	}, 5*time.Second, 20*time.Millisecond)

	cancel()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("cancel did not stop the running action")
	}
	proc, err := os.FindProcess(pid)
	require.NoError(t, err)
	assert.Eventually(t, func() bool { return proc.Signal(syscall.Signal(0)) != nil }, 5*time.Second, 20*time.Millisecond)
}

func TestExecCustomAction_BackgroundSignalKilledTitle(t *testing.T) {
	msg := runBackground(t, "echo hi; kill -9 $$")
	res, ok := msg.(customActionFailedMsg)
	require.True(t, ok, "got %T", msg)
	assert.Equal(t, "Label it failed: signal: killed", res.title)
}

func TestUpdateCustomActionFailed_OpensDescribeInExplorer(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer
	ret, _ := m.Update(customActionFailedMsg{owner: m.currentTabUID(), title: "Label it failed (exit 3)", output: "boom"})
	got := ret.(Model)
	assert.Equal(t, modeDescribe, got.mode)
	assert.Equal(t, "Label it failed (exit 3)", got.describeView.title)
	assert.Contains(t, stripANSI(got.describeView.content), "boom")
}

func TestUpdateCustomActionFailed_OutsideExplorerKeepsMode(t *testing.T) {
	m := basePush80Model()
	m.mode = modeLogs
	ret, _ := m.Update(customActionFailedMsg{owner: m.currentTabUID(), title: "Label it failed (exit 3)", output: "boom"})
	got := ret.(Model)
	assert.Equal(t, modeLogs, got.mode)
	assert.True(t, got.statusMessageErr)
	assert.Contains(t, got.statusMessage, "Label it failed (exit 3)")
}

func TestUpdateCustomActionFailed_OtherTabOrOverlayKeepsScreen(t *testing.T) {
	m := basePush80Model()
	m.mode = modeExplorer

	ret, _ := m.Update(customActionFailedMsg{owner: m.currentTabUID() + 1, title: "T failed (exit 1)", output: "boom"})
	got := ret.(Model)
	assert.Equal(t, modeExplorer, got.mode)
	assert.True(t, got.statusMessageErr)

	m.overlay = overlayAction
	ret, _ = m.Update(customActionFailedMsg{owner: m.currentTabUID(), title: "T failed (exit 1)", output: "boom"})
	got = ret.(Model)
	assert.Equal(t, modeExplorer, got.mode)
	assert.Equal(t, overlayAction, got.overlay)
	assert.True(t, got.statusMessageErr)
}

func TestUpdateCustomActionFailed_ActiveInputKeepsFocus(t *testing.T) {
	cases := map[string]func(*Model){
		"filter":     func(m *Model) { m.filterActive = true },
		"search":     func(m *Model) { m.searchActive = true },
		"commandbar": func(m *Model) { m.commandBarActive = true },
	}
	for name, set := range cases {
		t.Run(name, func(t *testing.T) {
			m := basePush80Model()
			m.mode = modeExplorer
			set(&m)
			ret, _ := m.Update(customActionFailedMsg{owner: m.currentTabUID(), title: "T failed (exit 1)", output: "boom"})
			got := ret.(Model)
			assert.Equal(t, modeExplorer, got.mode)
			assert.True(t, got.statusMessageErr)
		})
	}
}

func TestExecuteActionDefault_BackgroundRespectsReadOnly(t *testing.T) {
	prev := ui.ConfigCustomActions
	t.Cleanup(func() { ui.ConfigCustomActions = prev })
	ui.ConfigCustomActions = map[string][]ui.CustomAction{
		"Pod": {
			{Label: "Unsafe", Command: "true", Background: true},
			{Label: "Safe", Command: "true", Background: true, ReadOnlySafe: true},
		},
	}
	m := testModelExec()
	m.readOnly = true
	m.actionCtx.kind = "Pod"

	ret, _ := m.executeActionDefault("Unsafe")
	assert.Equal(t, readOnlyBlockedMessage("Unsafe"), ret.(Model).statusMessage)

	ret, _ = m.executeActionDefault("Safe")
	assert.NotEqual(t, readOnlyBlockedMessage("Safe"), ret.(Model).statusMessage)
}
