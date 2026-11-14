package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/janosmiko/lfk/internal/app/scheduler"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

const (
	maxCustomActionOutput = 1 << 20
	// customActionWaitDelay bounds the wait for output pipes that a
	// backgrounded grandchild (`foo &`) keeps open after sh exits.
	customActionWaitDelay = 2 * time.Second
	truncatedNotice       = "\n[output truncated]"
)

// customActionFailedMsg carries the output of a failed background custom action.
type customActionFailedMsg struct {
	owner  uint64
	title  string
	output string
}

// cappedBuffer keeps the first maxCustomActionOutput bytes and drops the rest.
type cappedBuffer struct {
	buf       bytes.Buffer
	truncated bool
}

func (c *cappedBuffer) Write(p []byte) (int, error) {
	room := maxCustomActionOutput - c.buf.Len()
	if len(p) > room {
		c.truncated = true
	}
	if room > 0 {
		c.buf.Write(p[:min(room, len(p))])
	}
	return len(p), nil
}

func (m Model) customActionContext() context.Context {
	if m.quitCtx != nil {
		return m.quitCtx
	}
	return context.Background()
}

func (m Model) backgroundCustomAction(ca ui.CustomAction, expandedCmd, kctx string, env []string) tea.Cmd {
	label := ca.Label
	owner := m.currentTabUID()
	cmd := exec.CommandContext(m.customActionContext(), "sh", "-c", expandedCmd)
	cmd.Env = env
	cmd.WaitDelay = customActionWaitDelay
	detachFromTerminal(cmd)
	return m.trackBgTask(scheduler.KindSubprocess, "Custom action: "+label, bgtaskTarget(kctx, m.actionCtx.namespace), func() tea.Msg {
		var out cappedBuffer
		cmd.Stdout, cmd.Stderr = &out, &out
		err := cmd.Run()
		// A leftover `foo &` holding the pipe returns ErrWaitDelay even when sh exited 0.
		if err == nil || (errors.Is(err, exec.ErrWaitDelay) && cmd.ProcessState.ExitCode() == 0) {
			return actionResultMsg{message: label + " completed"}
		}
		logger.Error("Custom action failed", "action", label, "context", kctx, "error", err)
		text := strings.TrimSpace(strings.ToValidUTF8(out.buf.String(), ""))
		if text == "" || cmd.ProcessState == nil {
			return actionResultMsg{err: fmt.Errorf("%s failed: %w", label, err)}
		}
		title := fmt.Sprintf("%s failed: %v", label, err)
		if code := cmd.ProcessState.ExitCode(); code >= 0 {
			title = fmt.Sprintf("%s failed (exit %d)", label, code)
		}
		text = sanitizeDescribeContent(cleanANSI(text))
		if out.truncated {
			text += truncatedNotice
		}
		return customActionFailedMsg{owner: owner, title: title, output: text}
	})
}
