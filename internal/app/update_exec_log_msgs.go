package app

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateExecPTYTick(msg execPTYTickMsg) (tea.Model, tea.Cmd) {
	if msg.ptmx != m.execPTY || m.mode != modeExec {
		// Check if a background tab owns this PTY — keep ticking for it.
		for i := range m.tabs {
			if i != m.activeTab && m.tabs[i].execPTY == msg.ptmx && m.tabs[i].execPTY != nil {
				ptmx := msg.ptmx
				return m, tea.Tick(50*time.Millisecond, func(t time.Time) tea.Msg {
					return execPTYTickMsg{ptmx: ptmx}
				})
			}
		}
		return m, nil
	}
	// Continue ticking to refresh the terminal view.
	return m, m.scheduleExecTick()
}

func (m Model) updateExecPTYExit(msg execPTYExitMsg) Model {
	if msg.ptmx == m.execPTY {
		m.execDone.Store(true)
	} else {
		// Mark the background tab's exec as done.
		for i := range m.tabs {
			if m.tabs[i].execPTY == msg.ptmx {
				m.tabs[i].execDone.Store(true)
				break
			}
		}
	}
	return m
}

func (m Model) updateExecPTYStart(msg execPTYStartMsg) (tea.Model, tea.Cmd) {
	m.execPTY = msg.ptmx
	m.execTerm = msg.term
	m.execTitle = msg.title
	m.execDone = &atomic.Bool{}
	m.execMu = &sync.Mutex{}
	// Configurable via ui.ConfigScrollbackLines (default
	// ui.ScrollbackLinesDefault = 5000) — generous for typical sessions,
	// bounded memory for long-running shells. The reader writes here in
	// lock-step with the vt10x terminal so scroll offsets line up.
	m.execScrollback = newScrollback(ui.ConfigScrollbackLines)
	m.execScrollOffset = 0
	m.mode = modeExec

	// Start background reader goroutine.
	startExecPTYReader(msg.ptmx, msg.term, m.execScrollback, msg.cmd, m.execMu, m.execDone)

	return m, m.scheduleExecTick()
}

func (m Model) updateLogLine(msg logLineMsg) (tea.Model, tea.Cmd) {
	if msg.ch != m.logCh {
		// Message from a background tab's log stream — buffer it into that tab's state.
		for i := range m.tabs {
			if m.tabs[i].logCh == msg.ch {
				if !msg.done {
					m.tabs[i].logLines = append(m.tabs[i].logLines, msg.line)
					// Continue draining: re-issue waitForLogLine for that channel.
					ch := msg.ch
					return m, func() tea.Msg {
						line, ok := <-ch
						if !ok {
							return logLineMsg{done: true, ch: ch}
						}
						return logLineMsg{line: line, ch: ch}
					}
				}
				break
			}
		}
		return m, nil
	}
	if msg.done {
		// When following all containers of a single Pod, the stream ends as
		// soon as the currently-running set of containers all exit. For a
		// pod still in its init phase that's every init container
		// transition — schedule an auto-reconnect so the next container
		// streams in without manual action. Bail out after
		// logAutoReconnectMaxAttempts consecutive empty reconnects so we
		// don't spin forever once the pod is truly terminated.
		if m.shouldAutoReconnectLogs() && m.logAutoReconnectAttempt < logAutoReconnectMaxAttempts {
			m.logAutoReconnectAttempt++
			return m, m.scheduleLogStreamRestart(msg.ch)
		}
		return m, nil
	}
	// A line arrived — the stream is producing output, so any pending
	// auto-reconnect backoff is no longer relevant.
	if m.logAutoReconnectAttempt > 0 {
		m.logAutoReconnectAttempt = 0
	}
	m.logLines = append(m.logLines, msg.line)
	if m.logFollow {
		m.logScroll, m.logWrapTopSkip = m.logMaxScrollAndSkip()
		m.logCursor = len(m.logLines) - 1
	}
	return m, m.waitForLogLine()
}

// shouldAutoReconnectLogs reports whether the log stream should automatically
// reconnect when it ends. Auto-reconnect is limited to single-Pod streams
// following all containers while the user is still in follow mode — that's
// the case where kubectl exits on every init-container transition.
// Specific-container, multi-pod, previous-logs, and non-Pod flows either
// have explicit end semantics (--previous) or use selector-based follows
// where "done" doesn't necessarily mean a transition. If the user has
// scrolled away from the tail (logFollow=false) they're reading history,
// not watching live — no point re-arming the stream on their behalf.
func (m Model) shouldAutoReconnectLogs() bool {
	return m.mode == modeLogs &&
		m.logFollow &&
		!m.logIsMulti &&
		!m.logPrevious &&
		m.actionCtx.kind == "Pod" &&
		m.actionCtx.containerName == ""
}

// updateLogStreamRestart fires when a scheduled auto-reconnect is due. If
// the user has switched pods, exited logs mode, or the stream has been
// replaced (e.g. by a manual action), the restart is silently dropped.
func (m Model) updateLogStreamRestart(msg logStreamRestartMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeLogs || m.logCh != msg.ch || !m.shouldAutoReconnectLogs() {
		return m, nil
	}
	m.logReconnecting = true
	cmd := m.startLogStream()
	m.logReconnecting = false
	return m, cmd
}

func (m Model) updateLogHistory(msg logHistoryMsg) Model {
	m.logLoadingHistory = false
	if msg.err != nil {
		m.logHasMoreHistory = false
		return m
	}
	if m.mode != modeLogs {
		return m
	}

	// Find overlap: search for the first 3 current lines in the fetched history.
	overlapIdx := -1
	if len(m.logLines) >= 3 && len(msg.lines) > 3 {
		first3 := m.logLines[:3]
		for i := len(msg.lines) - 3; i >= 0; i-- {
			if msg.lines[i] == first3[0] && msg.lines[i+1] == first3[1] && msg.lines[i+2] == first3[2] {
				overlapIdx = i
				break
			}
		}
	} else if len(m.logLines) > 0 && len(msg.lines) > 0 {
		// Single-line fallback.
		for i, line := range slices.Backward(msg.lines) {
			if line == m.logLines[0] {
				overlapIdx = i
				break
			}
		}
	}

	var newOlderLines []string
	if overlapIdx > 0 {
		newOlderLines = msg.lines[:overlapIdx]
	} else if overlapIdx == -1 && len(msg.lines) > 0 {
		// No overlap found; prepend all (logs may have rotated).
		newOlderLines = msg.lines
	}

	if len(newOlderLines) == 0 {
		m.logHasMoreHistory = false
		return m
	}

	// Prepend and adjust scroll to maintain view position.
	prepended := len(newOlderLines)
	m.logLines = append(newOlderLines, m.logLines...)
	m.logScroll += prepended
	if m.logCursor >= 0 {
		m.logCursor += prepended
	}
	m.logTailLines += ui.ConfigLogTailLines

	// Cap total to prevent unbounded growth.
	if m.logTailLines > 100000 {
		m.logHasMoreHistory = false
	}

	return m
}

func (m Model) updateLogSaveAll(msg logSaveAllMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setErrorFromErr("Log save failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	logger.Info("Saved all logs", "path", msg.path)
	m.setStatusMessage("All logs saved to "+msg.path+" (copied to clipboard)", false)
	return m, tea.Batch(copyToSystemClipboard(msg.path), scheduleStatusClear())
}
