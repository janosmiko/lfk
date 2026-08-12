package app

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/logger"
	"github.com/janosmiko/lfk/internal/ui"
)

func (m Model) updateExecPTYTick(msg execPTYTickMsg) (tea.Model, tea.Cmd) {
	// Drop ticks from superseded chains. Every tab switch / focus arms a fresh
	// generation; without this guard each armed chain self-perpetuates and they
	// accumulate one 50ms render loop per switch. Generation 0 means the counter
	// is uninitialized (test models), so skip the check there.
	if m.execTickGen != nil && msg.gen != m.execTickGen.Load() {
		return m, nil
	}
	// The chain only ticks for the focused exec PTY. Background tabs don't need
	// ticking — their reader goroutine keeps feeding vt10x regardless, and
	// switching back into the tab re-arms a fresh chain.
	if msg.ptmx != m.execPTY || m.mode != modeExec {
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
	// The reader that produced this message has exited (it does one receive then
	// returns). Mark the channel idle; the branches below re-arm exactly one
	// reader when the stream is still live, maintaining the single-reader
	// invariant the tab-switch guard relies on. Drop the entry entirely on
	// stream end so the map doesn't retain closed channels.
	if m.logReaderInFlight != nil {
		if msg.done {
			delete(m.logReaderInFlight, msg.ch)
		} else {
			m.logReaderInFlight[msg.ch] = false
		}
	}
	if msg.ch != m.logView.ch {
		// Message from a background tab's log stream — buffer it into that tab's state.
		for i := range m.tabs {
			if m.tabs[i].logCh == msg.ch {
				if !msg.done {
					m.tabs[i].logLines = append(m.tabs[i].logLines, msg.line)
					// Bound the background tab's buffer; shift its saved
					// offsets so restoring the tab lands on the same content.
					if trimmed, drop := capLogLines(m.tabs[i].logLines); drop > 0 {
						m.tabs[i].logLines = trimmed
						m.tabs[i].logScroll = shiftLogOffset(m.tabs[i].logScroll, drop)
						m.tabs[i].logCursor = shiftLogOffset(m.tabs[i].logCursor, drop)
						m.tabs[i].logVisualStart = shiftLogOffset(m.tabs[i].logVisualStart, drop)
						// scroll now points at a different source line; the
						// old sub-line skip no longer applies.
						m.tabs[i].logWrapTopSkip = 0
					}
					// Continue draining: re-arm a reader for that channel.
					return m, m.readLogChannel(msg.ch)
				}
				break
			}
		}
		return m, nil
	}
	if name, ok := strings.CutPrefix(msg.line, cronJobNoRunsSentinel); ok {
		m.setStatusMessage(fmt.Sprintf("No runs yet for CronJob %s", name), true)
		return m, tea.Batch(m.waitForLogLine(), scheduleStatusClear())
	}
	if msg.done {
		// When following all containers of a single Pod, the stream ends as
		// soon as the currently-running set of containers all exit. For a
		// pod still in its init phase that's every init container
		// transition — schedule an auto-reconnect so the next container
		// streams in without manual action. Bail out after
		// logAutoReconnectMaxAttempts consecutive empty reconnects so we
		// don't spin forever once the pod is truly terminated.
		if m.shouldAutoReconnectLogs() && m.logView.autoReconnectAttempt < logAutoReconnectMaxAttempts {
			m.logView.autoReconnectAttempt++
			return m, m.scheduleLogStreamRestart(msg.ch)
		}
		return m, nil
	}
	// A line arrived — the stream is producing output, so any pending
	// auto-reconnect backoff is no longer relevant. (Reset for transient
	// "waiting to start" notices too, so a slow image pull keeps polling past
	// the give-up cap instead of stalling.)
	if m.logView.autoReconnectAttempt > 0 {
		m.logView.autoReconnectAttempt = 0
	}
	if isKubectlTransientError(msg.line) {
		// Container hasn't started yet. Flag the pending start so a
		// specific-container stream reconnects when it ends, and show the
		// notice only once — not one copy per reconnect poll.
		if m.logView.pendingContainerStart {
			return m, m.waitForLogLine()
		}
		m.logView.pendingContainerStart = true
	} else {
		m.logView.pendingContainerStart = false
	}
	if m.mode == modeLogTop {
		m.ingestLogTopLine(msg.line)
	} else {
		m.appendRawLogLine(msg.line)
	}
	// Bound the live buffer so a long-running follow doesn't grow memory
	// without limit (issue #387). Trim the raw stream; offsets index the
	// displayed projection, so without a filter we shift them by the dropped
	// count, and with a filter we re-project (offsets re-clamped there).
	if trimmed, drop := capLogLines(m.logView.rawLines); drop > 0 {
		// Count how many of the dropped raw lines were visible under the active
		// filter before trimming, so we shift the projected offsets by the
		// VISIBLE delta (not the raw drop count).
		visibleDrop := 0
		if m.logFilterActive() {
			visibleDrop = m.countVisibleRaw(m.logView.rawLines[:drop])
		}
		m.logView.rawLines = trimmed
		if len(m.logView.rawSev) > 0 {
			cut := min(drop, len(m.logView.rawSev))
			retained := make([]int, len(m.logView.rawSev)-cut)
			copy(retained, m.logView.rawSev[cut:])
			m.logView.rawSev = retained
		}
		if m.logFilterActive() {
			// Shift by the visible delta so a non-following viewport keeps its
			// place; follow mode re-pins to the bottom below regardless.
			m.logView.scroll = shiftLogOffset(m.logView.scroll, visibleDrop)
			m.logView.cursor = shiftLogOffset(m.logView.cursor, visibleDrop)
			m.logView.visualStart = shiftLogOffset(m.logView.visualStart, visibleDrop)
			m.logView.wrapTopSkip = 0
			m.rebuildLogView()
		} else {
			m.logView.lines = m.logView.rawLines
			m.logView.scroll = shiftLogOffset(m.logView.scroll, drop)
			m.logView.cursor = shiftLogOffset(m.logView.cursor, drop)
			m.logView.visualStart = shiftLogOffset(m.logView.visualStart, drop)
			// scroll now points at a different source line; the old sub-line
			// skip no longer applies (follow mode recomputes it just below).
			m.logView.wrapTopSkip = 0
		}
	}
	if m.logView.follow {
		m.logView.scroll, m.logView.wrapTopSkip = m.logMaxScrollAndSkip()
		m.logView.cursor = len(m.logView.lines) - 1
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
	if m.mode != modeLogs || !m.logView.follow || m.logView.isMulti ||
		m.logView.previous || m.actionCtx.kind != "Pod" {
		return false
	}
	if m.actionCtx.containerName == "" {
		// All-containers single-Pod stream: kubectl exits on every
		// init-container transition; reconnect to pick up the next one.
		return true
	}
	// A specific container was selected. kubectl logs -c <name> exits
	// immediately with a "waiting to start" error while the container is
	// ContainerCreating/PodInitializing. Reconnect only while that pending
	// state holds; once it has produced real output, a normal stream-end is
	// terminal (the user opted into that one stream).
	return m.logView.pendingContainerStart
}

// updateLogStreamRestart fires when a scheduled auto-reconnect is due. If
// the user has switched pods, exited logs mode, or the stream has been
// replaced (e.g. by a manual action), the restart is silently dropped.
func (m Model) updateLogStreamRestart(msg logStreamRestartMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeLogs || m.logView.ch != msg.ch || !m.shouldAutoReconnectLogs() {
		return m, nil
	}
	m.logView.reconnecting = true
	cmd := m.startLogStream()
	m.logView.reconnecting = false
	return m, cmd
}

func (m Model) updateLogHistory(msg logHistoryMsg) (Model, tea.Cmd) {
	m.logView.loadingHistory = false
	if msg.err != nil {
		m.logView.hasMoreHistory = false
		if errors.Is(msg.err, errCronJobNoRuns) {
			m.setStatusMessage("No runs yet for CronJob "+m.actionCtx.name, true)
			return m, scheduleStatusClear()
		}
		return m, nil
	}
	if m.mode != modeLogs {
		return m, nil
	}

	// The fetched history is the last <tail> lines of the resource; the current
	// buffer's oldest lines overlap its tail, so only the prefix before the
	// overlap is genuinely older.
	newOlderLines := mergeOlderLogLines(m.logView.rawLines, msg.lines)

	if len(newOlderLines) == 0 {
		m.logView.hasMoreHistory = false
		return m, nil
	}

	// Prepend and adjust scroll to maintain view position — unless the user
	// is still anchored at the absolute top (e.g. just pressed `gg` and the
	// async fetch resolved before they navigated), in which case keep them
	// at 0 so the newly revealed older lines come into view.
	prepended := len(newOlderLines)
	// How many prepended lines are visible under the active filter (the amount
	// the projected offsets must grow by); raw count otherwise.
	visiblePrepended := prepended
	if m.logFilterActive() {
		visiblePrepended = m.countVisibleRaw(newOlderLines)
	}
	m.logView.rawLines = append(newOlderLines, m.logView.rawLines...)
	m.logView.rawSev = nil // ordering changed; recompute lazily when needed
	if m.logView.cursor > 0 || m.logView.scroll > 0 {
		m.logView.scroll += visiblePrepended
		if m.logView.cursor >= 0 {
			m.logView.cursor += visiblePrepended
		}
	}
	if m.logFilterActive() {
		m.rebuildLogView()
	} else {
		m.logView.lines = m.logView.rawLines
	}
	m.logView.tailLines += ui.ConfigLogTailLines

	// Cap total to prevent unbounded growth.
	if m.logView.tailLines > 100000 {
		m.logView.hasMoreHistory = false
	}

	return m, nil
}

func (m Model) updateLogSaveAll(msg logSaveAllMsg) (tea.Model, tea.Cmd) {
	if errors.Is(msg.err, errCronJobNoRuns) {
		m.setStatusMessage("No runs yet for CronJob "+m.actionCtx.name, true)
		return m, scheduleStatusClear()
	}
	if msg.err != nil {
		m.setErrorFromErr("Log save failed: ", msg.err)
		return m, scheduleStatusClear()
	}
	logger.Info("Saved all logs", "path", msg.path)
	m.setStatusMessage("All logs saved to "+msg.path+" (copied to clipboard)", false)
	return m, tea.Batch(copyToSystemClipboard(msg.path), scheduleStatusClear())
}
