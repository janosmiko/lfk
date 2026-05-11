package app

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- clearBeforeExec ---

func TestClearBeforeExec(t *testing.T) {
	t.Run("wraps simple command", func(t *testing.T) {
		original := exec.Command("kubectl", "get", "pods")
		wrapped := clearBeforeExecForOS(original, "linux")

		assert.Equal(t, "sh", wrapped.Path[len(wrapped.Path)-2:])
		assert.Equal(t, "sh", wrapped.Args[0])
		assert.Equal(t, "-c", wrapped.Args[1])
		assert.Contains(t, wrapped.Args[2], "printf")
		assert.Contains(t, wrapped.Args[2], "'kubectl'")
		assert.Contains(t, wrapped.Args[2], "'get'")
		assert.Contains(t, wrapped.Args[2], "'pods'")
	})

	t.Run("preserves environment", func(t *testing.T) {
		original := exec.Command("kubectl", "get", "pods")
		original.Env = []string{"KUBECONFIG=/tmp/config"}
		original.Dir = "/some/dir"
		wrapped := clearBeforeExecForOS(original, "linux")

		assert.Equal(t, []string{"KUBECONFIG=/tmp/config"}, wrapped.Env)
		assert.Equal(t, "/some/dir", wrapped.Dir)
	})

	t.Run("quotes args with special chars", func(t *testing.T) {
		original := exec.Command("kubectl", "get", "pods", "-l", "app=my app")
		wrapped := clearBeforeExecForOS(original, "linux")

		assert.Contains(t, wrapped.Args[2], "'app=my app'")
	})

	t.Run("handles args with single quotes", func(t *testing.T) {
		original := exec.Command("echo", "it's")
		wrapped := clearBeforeExecForOS(original, "linux")

		// shellQuote replaces ' with '"'"'
		assert.Contains(t, wrapped.Args[2], `'it'"'"'s'`)
	})
}

// On Windows, sh.exe is not on a standard PATH, so wrapping a kubectl
// command in `sh -c "printf '\033c' && exec kubectl …"` makes the parent
// process exit immediately — that's what bug #194 surfaced as
// "exit status 2" with the terminal flashing and closing. Returning the
// original cmd unchanged on Windows lets tea.ExecProcess hand kubectl
// the host terminal directly, losing only the cosmetic clear-screen.
func TestClearBeforeExecForOS(t *testing.T) {
	t.Run("linux wraps with sh -c", func(t *testing.T) {
		cmd := exec.Command("kubectl", "exec", "-it", "pod")
		wrapped := clearBeforeExecForOS(cmd, "linux")
		assert.Equal(t, "sh", wrapped.Args[0])
		assert.Equal(t, "-c", wrapped.Args[1])
		assert.Contains(t, wrapped.Args[2], `printf '\033c'`)
	})

	t.Run("darwin wraps with sh -c", func(t *testing.T) {
		cmd := exec.Command("kubectl", "exec", "-it", "pod")
		wrapped := clearBeforeExecForOS(cmd, "darwin")
		assert.Equal(t, "sh", wrapped.Args[0])
	})

	t.Run("windows returns cmd unchanged because sh is not reliably available", func(t *testing.T) {
		cmd := exec.Command("kubectl", "exec", "-it", "pod")
		wrapped := clearBeforeExecForOS(cmd, "windows")
		assert.Same(t, cmd, wrapped, "windows path must not wrap — sh -c would fail before kubectl ever starts")
	})
}

// --- SetVersion ---

func TestSetVersion(t *testing.T) {
	m := Model{}
	m.SetVersion("1.2.3")
	assert.Equal(t, "1.2.3", m.version)
}

func TestSetVersionOverwrite(t *testing.T) {
	m := Model{}
	m.SetVersion("1.0.0")
	m.SetVersion("2.0.0")
	assert.Equal(t, "2.0.0", m.version)
}

func TestSetVersionEmpty(t *testing.T) {
	m := Model{}
	m.SetVersion("")
	assert.Equal(t, "", m.version)
}

// --- SetStderrChan ---

func TestSetStderrChan(t *testing.T) {
	m := Model{}
	ch := make(chan string, 1)
	m.SetStderrChan(ch)
	assert.NotNil(t, m.stderrChan)
}

func TestSetStderrChanNil(t *testing.T) {
	m := Model{}
	m.SetStderrChan(nil)
	assert.Nil(t, m.stderrChan)
}

// --- scheduleStatusClear ---

func TestScheduleStatusClear(t *testing.T) {
	cmd := scheduleStatusClear()
	assert.NotNil(t, cmd)
}

// --- scheduleStartupTip ---

func TestScheduleStartupTip(t *testing.T) {
	cmd := scheduleStartupTip()
	assert.NotNil(t, cmd)
}

// --- scheduleWatchTick ---

func TestScheduleWatchTick(t *testing.T) {
	cmd := scheduleWatchTick(5)
	assert.NotNil(t, cmd)
}

// --- startupTips ---

func TestStartupTipsHasEntries(t *testing.T) {
	assert.True(t, len(startupTips) > 0)
}
