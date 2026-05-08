package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func TestExecuteActionCaptureTraffic_PodOpensConfigPhase(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.actionCtx = actionContext{
		kind: "Pod", name: "pod1", namespace: "ns", context: "test-ctx",
		resourceType: model.ResourceTypeEntry{Kind: "Pod", Resource: "pods", Namespaced: true},
	}

	out, _ := m.executeActionCaptureTraffic()
	mm := out.(Model)

	if mm.overlay != overlayTrafficCapture {
		t.Errorf("overlay = %v, want overlayTrafficCapture", mm.overlay)
	}
	if mm.captureOverlay.phase != capturePhaseConfig {
		t.Errorf("phase = %v, want capturePhaseConfig (Pod target skips endpoint pick)", mm.captureOverlay.phase)
	}
	if mm.captureOverlay.targetName != "pod1" {
		t.Errorf("targetName = %q, want pod1", mm.captureOverlay.targetName)
	}
}

func TestExecuteActionCaptureTraffic_ServiceOpensEndpointPick(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	m.actionCtx = actionContext{
		kind: "Service", name: "svc1", namespace: "ns", context: "test-ctx",
		resourceType: model.ResourceTypeEntry{Kind: "Service", Resource: "services", Namespaced: true},
	}

	out, _ := m.executeActionCaptureTraffic()
	mm := out.(Model)

	if mm.captureOverlay.phase != capturePhaseEndpointPick {
		t.Errorf("phase = %v, want capturePhaseEndpointPick", mm.captureOverlay.phase)
	}
}
