package app

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

// ForceQuitGracePeriod bounds how long the graceful drain may run before
// the force-quit watchdog (armed via SetShutdownNotifier) kills the
// program. Exported so main can wire the watchdog to the same value the
// shutdown notice advertises.
const ForceQuitGracePeriod = 10 * time.Second

// shutdownState groups the graceful-quit fields so they live together
// without pushing the main Model struct over the file-length cap (the
// same pattern whoCanState uses). It is embedded in Model, so the quit
// handlers reference its fields directly (m.shuttingDown, m.shutdownNotifier).
type shutdownState struct {
	// shuttingDown is set once the user commits to quitting. It blocks
	// further input and switches the view to the graceful-shutdown notice
	// while background workers drain.
	shuttingDown bool
	// shutdownNotifier, when set, is invoked the moment graceful shutdown
	// begins. main wires it to a watchdog that force-exits the process if
	// the drain hangs past ForceQuitGracePeriod.
	shutdownNotifier func()
}

// SetShutdownNotifier registers a callback invoked once when graceful
// shutdown begins. main uses it to arm a force-exit watchdog so a hung
// background drain cannot wedge the terminal indefinitely.
func (m *Model) SetShutdownNotifier(fn func()) {
	m.shutdownNotifier = fn
}

// beginShutdown starts a graceful quit: it fires the instant cancellations
// synchronously (so in-flight API requests abort before they ride out TCP
// timeouts), shows the shutdown notice, notifies the force-exit watchdog,
// and hands the blocking drain off to a command that runs on its own
// goroutine. The drain reports completion via shutdownCompleteMsg, at
// which point Update dispatches tea.Quit.
func (m Model) beginShutdown() (tea.Model, tea.Cmd) {
	if m.shuttingDown {
		return m, nil
	}
	// Arm the force-quit watchdog first so the grace period bounds every
	// step below, including the synchronous session save — a local-disk
	// write that is normally fast but could stall on a slow or networked
	// home directory.
	if m.shutdownNotifier != nil {
		m.shutdownNotifier()
	}
	m.signalShutdown()
	// Persist the session synchronously on the UI goroutine: keeping it off
	// the drain goroutine avoids racing with a late Update-driven save.
	m.saveCurrentSession()
	m.overlay = overlayShuttingDown
	m.shuttingDown = true
	return m, m.shutdownDrainCmd()
}

// signalShutdown issues every non-blocking cancellation so background
// goroutines (port-forwards, captures, log streams, exec PTYs, in-flight
// API requests) begin winding down immediately. Every call here only signals
// — nothing waits — so it is safe to run on the UI goroutine without
// freezing the render loop. Centralising it keeps the quit entry points
// (handleQuitConfirmOverlayKey, closeTabOrQuit's last-tab branch, and the
// :quit command) in lockstep.
func (m *Model) signalShutdown() {
	if m.portForwardMgr != nil {
		m.portForwardMgr.StopAll()
	}
	if m.captureMgr != nil {
		m.captureMgr.StopAll()
	}
	m.cancelAllTabLogStreams()
	m.closeAllExecPTYs()
	m.cancelInFlightRequests()
}

// shutdownDrainCmd runs drainShutdown on a background goroutine and emits
// shutdownCompleteMsg when it returns. It closes over a snapshot of the
// model; that is safe because input is blocked once shuttingDown is set,
// so no concurrent Update mutates the state drainShutdown reads.
func (m Model) shutdownDrainCmd() tea.Cmd {
	return func() tea.Msg {
		m.drainShutdown()
		return shutdownCompleteMsg{}
	}
}

// drainShutdown blocks until the worker pools and informer caches have
// fully stopped. Each wait here can take seconds when a cluster is
// unreachable, so it runs off the UI goroutine (see shutdownDrainCmd)
// behind the graceful-shutdown notice. signalShutdown must have run first
// so the goroutines this waits on are already cancelled and exit promptly.
func (m *Model) drainShutdown() {
	if m.scheduler != nil {
		m.scheduler.StopWorkers()
	}
	if m.client != nil {
		m.client.Shutdown()
	}
}
