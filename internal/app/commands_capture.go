package app

import (
	"context"
	"fmt"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
	"github.com/janosmiko/lfk/internal/ui"
)

// captureBackendsLoadedMsg is dispatched when the async kubeshark probe
// resolves. backend is the kubeshark availability row to APPEND to the
// existing backends slice (or the zero value if kubeshark isn't deployed
// — in that case the chip is omitted entirely). kubectl-debug is set
// synchronously by executeActionCaptureTraffic and is not in this message.
type captureBackendsLoadedMsg struct {
	backend   captureBackendAvailability
	kubeshark *k8s.KubesharkInfo
}

// loadCaptureBackends probes kubeshark (Service get on the configured
// namespace) and returns the result. kubectl-debug is NOT probed here —
// executeActionCaptureTraffic populates it synchronously so the user sees
// the chip the instant the overlay opens, instead of staring at an empty
// backend row while a remote-cluster API call resolves.
//
// Uses m.reqCtx so a navigation change while the probe is in flight
// cancels it instead of running to completion.
func (m Model) loadCaptureBackends() tea.Cmd {
	client := m.client
	kubectx := m.actionCtx.context
	ctx := m.reqCtx
	return func() tea.Msg {
		ks, ksErr := client.DetectKubeshark(ctx, kubectx)
		var backend captureBackendAvailability
		switch {
		case ks != nil:
			backend = captureBackendAvailability{Backend: k8s.BackendKubeshark, Available: true}
		case ksErr != nil:
			backend = captureBackendAvailability{
				Backend: k8s.BackendKubeshark, Available: false,
				Reason: "detection failed: " + ksErr.Error(),
			}
		default:
			// (nil, nil) means "kubeshark hub Service not deployed in the
			// configured namespace". Don't surface a backend chip — there's
			// nothing actionable for the user.
			return captureBackendsLoadedMsg{}
		}
		return captureBackendsLoadedMsg{backend: backend, kubeshark: ks}
	}
}

// captureStartedMsg is dispatched when a capture has been successfully started.
type captureStartedMsg struct {
	id      int
	request k8s.CaptureRequest
}

// captureFailedMsg is dispatched when a capture fails to start.
type captureFailedMsg struct {
	err error
}

// captureStoppedMsg is dispatched after a capture is stopped.
type captureStoppedMsg struct {
	id int
}

// captureLiveTickMsg triggers a redraw of the live packet view.
type captureLiveTickMsg struct {
	id int
}

// kubesharkLaunchedMsg is dispatched after kubeshark port-forward + browser open.
type kubesharkLaunchedMsg struct {
	err error
}

// captureUpdateMsg is dispatched whenever CaptureManager emits a state-change
// notification (Starting -> Running -> Stopped/Failed). It drives the
// __captures__ pseudo-resource auto-refresh, mirroring the port-forward
// pattern.
type captureUpdateMsg struct{}

// waitForCaptureUpdate registers a manager-level update callback and returns a
// tea.Cmd that blocks until the next state change. The handler is responsible
// for re-arming via another waitForCaptureUpdate() — same pattern as
// waitForPortForwardUpdate.
func (m Model) waitForCaptureUpdate() tea.Cmd {
	if m.captureMgr == nil {
		return nil
	}
	ch := make(chan struct{}, 1)
	m.captureMgr.SetUpdateCallback(func() {
		select {
		case ch <- struct{}{}:
		default:
		}
	})
	return func() tea.Msg {
		<-ch
		return captureUpdateMsg{}
	}
}

// startCapture starts a packet capture via CaptureManager and wires the
// onPacket callback to the supplied ring buffer. The caller MUST assign the
// same liveBuf to m.captureOverlay.liveBuf on the model it returns; the
// renderer reads from there for the live packet table. Earlier the assignment
// happened inside this function, but `m` is a value copy and the assignment
// was silently dropped — leaving the renderer reading from a nil ring while
// the counters (which read atomic fields on CaptureEntry) updated normally.
//
// Uses context.Background() rather than m.reqCtx because the capture is meant
// to outlive the current navigation — the user explicitly stops it via the
// overlay's `s` key (mgr.Stop) or via __captures__ row actions, and on app
// shutdown via captureMgr.StopAll() in performQuitCleanup. m.reqCtx would
// kill the capture every time the user navigates to a different view.
func (m Model) startCapture(req k8s.CaptureRequest, liveBuf *captureRing) tea.Cmd {
	mgr := m.captureMgr
	return func() tea.Msg {
		id, err := mgr.Start(context.Background(), req, func(s k8s.PacketSummary) {
			liveBuf.Push(s)
		})
		if err != nil {
			return captureFailedMsg{err: err}
		}
		return captureStartedMsg{id: id, request: req}
	}
}

// stopCapture stops an active capture by ID.
func (m Model) stopCapture(id int) tea.Cmd {
	mgr := m.captureMgr
	return func() tea.Msg {
		_ = mgr.Stop(id)
		return captureStoppedMsg{id: id}
	}
}

// launchKubeshark port-forwards the kubeshark hub and opens the browser.
//
// Uses m.reqCtx so the up-to-8-second wait for the port-forward to reach
// Running aborts immediately if the user navigates away or quits, rather
// than blocking the goroutine until the deadline. The port-forward itself
// is owned by PortForwardManager and outlives this call regardless.
func (m Model) launchKubeshark(target model.Item) tea.Cmd {
	client := m.client
	kubectx := m.actionCtx.context
	mgr := m.portForwardMgr
	ctx := m.reqCtx
	return func() tea.Msg {
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			return kubesharkLaunchedMsg{err: fmt.Errorf("kubectl not found: %w", err)}
		}
		err = client.LaunchKubeshark(ctx, kubectx, target.Namespace, target.Name, mgr, kubectlPath, ui.OpenBrowser)
		return kubesharkLaunchedMsg{err: err}
	}
}
