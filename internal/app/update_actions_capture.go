package app

import (
	"os"
	"strconv"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// executeActionCaptureTraffic opens the traffic capture overlay and
// dispatches async kubeshark detection. Mirrors executeActionCrashInvestigator.
//
// kubectl-debug is populated synchronously here so the user sees an
// immediately-usable backend chip the moment the overlay opens. The
// kubeshark probe runs in the background (a K8s API call that can take
// 200-2000ms on remote clusters) and appends to backends when it lands.
func (m Model) executeActionCaptureTraffic() (tea.Model, tea.Cmd) {
	m.captureOverlay = captureOverlayState{
		targetKind: m.actionCtx.kind,
		targetNS:   m.actionCtx.namespace,
		targetName: m.actionCtx.name,
		iface:      "any",
		snaplen:    65535,
		// kubectl-debug is always-available at probe time; failures
		// surface at start time via stderr translation.
		backends: []captureBackendAvailability{
			{Backend: k8s.BackendKubectlDebug, Available: true},
		},
		selectedBackend: k8s.BackendKubectlDebug,
	}
	if m.actionCtx.kind == "Service" {
		m.captureOverlay.phase = capturePhaseEndpointPick
	} else {
		m.captureOverlay.phase = capturePhaseConfig
	}
	m.overlay = overlayTrafficCapture
	return m, m.loadCaptureBackends()
}

// getCaptureIDFromItem extracts the capture ID from a row item's Extra field.
// Extra holds the ID as a plain decimal string (set by capturesPseudoItems).
func getCaptureIDFromItem(it model.Item) (int, bool) {
	id, err := strconv.Atoi(it.Extra)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// openCaptureFromPseudo re-opens the overlay attached to an existing capture.
// Called from the __captures__ row action menu ("Open").
func (m Model) openCaptureFromPseudo(it model.Item) (tea.Model, tea.Cmd) {
	id, ok := getCaptureIDFromItem(it)
	if !ok {
		m.setStatusMessage("invalid capture ID: "+it.Extra, true)
		return m, scheduleStatusClear()
	}
	if m.captureMgr == nil {
		return m, nil
	}
	for _, e := range m.captureMgr.Entries() {
		if e.ID == id {
			m.captureOverlay = captureOverlayState{
				targetKind: "Pod",
				targetNS:   e.Request.Namespace,
				targetName: e.Request.PodName,
				captureID:  id,
				liveBuf:    newCaptureRing(500),
			}
			switch e.Status {
			case k8s.CaptureStopped, k8s.CaptureFailed:
				m.captureOverlay.phase = capturePhaseStopped
			default:
				m.captureOverlay.phase = capturePhaseLive
			}
			m.overlay = overlayTrafficCapture
			return m, nil
		}
	}
	m.setStatusMessage("capture id "+it.Extra+" not found", true)
	return m, scheduleStatusClear()
}

// stopCaptureFromPseudo stops a running capture from the __captures__ pseudo-resource.
// Called from the row action menu ("Stop").
func (m Model) stopCaptureFromPseudo(it model.Item) (tea.Model, tea.Cmd) {
	id, ok := getCaptureIDFromItem(it)
	if !ok {
		m.setStatusMessage("invalid capture ID: "+it.Extra, true)
		return m, scheduleStatusClear()
	}
	return m, m.stopCapture(id)
}

// deleteCaptureFile removes the on-disk pcap file for a (typically stopped) capture.
// Called from the row action menu ("Delete File").
func (m Model) deleteCaptureFile(it model.Item) (tea.Model, tea.Cmd) {
	id, ok := getCaptureIDFromItem(it)
	if !ok {
		m.setStatusMessage("invalid capture ID: "+it.Extra, true)
		return m, scheduleStatusClear()
	}
	if m.captureMgr == nil {
		return m, nil
	}
	for _, e := range m.captureMgr.Entries() {
		if e.ID == id {
			if err := os.Remove(e.OutputPath); err != nil {
				m.setStatusMessage("delete failed: "+err.Error(), true)
				return m, scheduleStatusClear()
			}
			m.setStatusMessage("deleted "+e.OutputPath, false)
			return m, scheduleStatusClear()
		}
	}
	m.setStatusMessage("capture id "+it.Extra+" not found", true)
	return m, scheduleStatusClear()
}
