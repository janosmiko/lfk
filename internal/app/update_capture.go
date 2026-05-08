package app

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/model"
)

//nolint:unparam // tea.Cmd return is part of the msg-handler convention; may carry cmds in future
func (m Model) updateCaptureBackendsLoaded(msg captureBackendsLoadedMsg) (tea.Model, tea.Cmd) {
	m.captureOverlay.kubeshark = msg.kubeshark
	if msg.backend.Backend == "" {
		// Empty message: kubeshark hub Service not deployed in the
		// configured namespace. Leave the backends slice as it was set
		// synchronously by executeActionCaptureTraffic (kubectl-debug only).
		return m, nil
	}
	// Avoid duplicates: if the kubeshark row was already appended (e.g.
	// the probe somehow fires twice), replace it in place.
	for i, b := range m.captureOverlay.backends {
		if b.Backend == msg.backend.Backend {
			m.captureOverlay.backends[i] = msg.backend
			return m, nil
		}
	}
	m.captureOverlay.backends = append(m.captureOverlay.backends, msg.backend)
	return m, nil
}

func (m Model) updateCaptureStarted(msg captureStartedMsg) (tea.Model, tea.Cmd) {
	// Restart path: if the previous capture in this session was never
	// marked saved (no Y), delete its pcap now — the user is intentionally
	// abandoning it by starting a fresh capture.
	m.deleteUnsavedCapture()
	m.captureOverlay.unsavedCaptureID = msg.id
	m.captureOverlay.captureID = msg.id
	m.captureOverlay.phase = capturePhaseLive
	// Reset the scroll position so the new capture starts in tail -f mode
	// (latest at bottom). Old position would otherwise pin the user to
	// "rows back from the prior capture's last packet", which is not what
	// they want.
	m.captureOverlay.scrollOffset = 0
	return m, tea.Batch(m.scheduleCaptureTick(msg.id), m.waitForCaptureUpdate())
}

//nolint:unparam // tea.Cmd return is part of the msg-handler convention; may carry cmds in future
func (m Model) updateCaptureFailed(msg captureFailedMsg) (tea.Model, tea.Cmd) {
	m.setStatusMessage("capture failed: "+msg.err.Error(), true)
	m.captureOverlay.phase = capturePhaseConfig
	return m, nil
}

//nolint:unparam // tea.Cmd return is part of the msg-handler convention; may carry cmds in future
func (m Model) updateCaptureStopped(msg captureStoppedMsg) (tea.Model, tea.Cmd) {
	if m.captureOverlay.captureID == msg.id {
		m.captureOverlay.phase = capturePhaseStopped
	}
	return m, nil
}

func (m Model) updateCaptureLiveTick(msg captureLiveTickMsg) (tea.Model, tea.Cmd) {
	if m.captureOverlay.phase != capturePhaseLive || m.captureOverlay.captureID != msg.id {
		return m, nil
	}
	return m, m.scheduleCaptureTick(msg.id)
}

//nolint:unparam // tea.Cmd return is part of the msg-handler convention; may carry cmds in future
func (m Model) updateKubesharkLaunched(msg kubesharkLaunchedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.setStatusMessage("kubeshark: "+msg.err.Error(), true)
	} else {
		m.setStatusMessage("kubeshark hub opened in browser", false)
	}
	m.overlay = overlayNone
	return m, nil
}

// routeCaptureMsg dispatches all traffic-capture messages.
// Extracted from updateActionResultMsg to reduce its cyclomatic complexity.
func (m Model) routeCaptureMsg(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case captureBackendsLoadedMsg:
		return m.updateCaptureBackendsLoaded(msg)
	case captureStartedMsg:
		return m.updateCaptureStarted(msg)
	case captureFailedMsg:
		return m.updateCaptureFailed(msg)
	case captureStoppedMsg:
		return m.updateCaptureStopped(msg)
	case captureLiveTickMsg:
		return m.updateCaptureLiveTick(msg)
	case kubesharkLaunchedMsg:
		return m.updateKubesharkLaunched(msg)
	case captureUpdateMsg:
		return m.updateCaptureUpdate(msg)
	}
	return m, nil
}

// updateCaptureUpdate refreshes the __captures__ pseudo-resource view (if
// active) on a CaptureManager state change and re-arms the wait so subsequent
// changes also trigger a refresh.
//
//nolint:unparam // tea.Cmd return is part of the msg-handler convention
func (m Model) updateCaptureUpdate(_ captureUpdateMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{m.waitForCaptureUpdate()}
	if m.nav.Level == model.LevelResources && m.nav.ResourceType.Kind == "__captures__" {
		m.setMiddleItems(capturesPseudoItems(m.captureMgr))
		m.clampCursor()
	}
	return m, tea.Batch(cmds...)
}

func (m Model) scheduleCaptureTick(id int) tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return captureLiveTickMsg{id: id}
	})
}
