package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
)

// TestUpdateCaptureBackendsLoaded_AppendsKubeshark guards the new flow:
// kubectl-debug is set synchronously by executeActionCaptureTraffic; the
// async probe message only carries the kubeshark row to APPEND.
func TestUpdateCaptureBackendsLoaded_AppendsKubeshark(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig
	m.captureOverlay.backends = []captureBackendAvailability{
		{Backend: k8s.BackendKubectlDebug, Available: true},
	}

	msg := captureBackendsLoadedMsg{
		backend: captureBackendAvailability{Backend: k8s.BackendKubeshark, Available: true},
	}
	out, _ := m.updateCaptureBackendsLoaded(msg)
	mm := out.(Model)

	if len(mm.captureOverlay.backends) != 2 {
		t.Errorf("backends len = %d, want 2 (kubectl-debug + appended kubeshark)", len(mm.captureOverlay.backends))
	}
}

// TestUpdateCaptureBackendsLoaded_NoKubesharkLeavesBackendsAlone: when the
// kubeshark probe returns "not deployed" (zero-value backend in the msg),
// the synchronously-set backends array must be preserved as-is.
func TestUpdateCaptureBackendsLoaded_NoKubesharkLeavesBackendsAlone(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.backends = []captureBackendAvailability{
		{Backend: k8s.BackendKubectlDebug, Available: true},
	}

	out, _ := m.updateCaptureBackendsLoaded(captureBackendsLoadedMsg{})
	mm := out.(Model)

	if len(mm.captureOverlay.backends) != 1 {
		t.Errorf("backends len = %d, want 1 (kubectl-debug preserved)", len(mm.captureOverlay.backends))
	}
}

func TestUpdateCaptureStarted_FlipsToLivePhase(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseConfig

	out, cmd := m.updateCaptureStarted(captureStartedMsg{id: 42})
	mm := out.(Model)
	if mm.captureOverlay.phase != capturePhaseLive {
		t.Errorf("phase = %v, want capturePhaseLive", mm.captureOverlay.phase)
	}
	if mm.captureOverlay.captureID != 42 {
		t.Errorf("captureID = %d, want 42", mm.captureOverlay.captureID)
	}
	if cmd == nil {
		t.Errorf("expected scheduleCaptureTick cmd, got nil")
	}
}

func TestUpdateCaptureStopped_FlipsToStoppedPhase(t *testing.T) {
	m := baseFinalModel()
	m.overlay = overlayTrafficCapture
	m.captureOverlay.phase = capturePhaseLive
	m.captureOverlay.captureID = 42

	out, _ := m.updateCaptureStopped(captureStoppedMsg{id: 42})
	mm := out.(Model)
	if mm.captureOverlay.phase != capturePhaseStopped {
		t.Errorf("phase = %v, want capturePhaseStopped", mm.captureOverlay.phase)
	}
}
