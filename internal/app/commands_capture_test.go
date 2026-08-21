package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

// TestLoadCaptureBackends_KubesharkOnlyOnEmptyCluster guards that the async
// probe returns an empty msg (no chip to append) when the kubeshark hub
// Service isn't deployed. kubectl-debug is set synchronously by
// executeActionCaptureTraffic and is NOT in this message.
func TestLoadCaptureBackends_KubesharkOnlyOnEmptyCluster(t *testing.T) {
	m := baseFinalModel()
	m.actionCtx = actionContext{kind: "Pod", name: "pod1", namespace: "ns", context: "test-ctx"}

	raw := m.loadCaptureBackends()()
	msg, ok := raw.(captureBackendsLoadedMsg)
	if !ok {
		t.Fatalf("loadCaptureBackends returned %T, want captureBackendsLoadedMsg (raw=%+v)", raw, raw)
	}

	if msg.backend.Backend != "" {
		t.Errorf("expected empty backend (kubeshark not deployed); got %+v", msg.backend)
	}
	if msg.kubeshark != nil {
		t.Errorf("expected nil KubesharkInfo; got %+v", msg.kubeshark)
	}
}

func TestStartCapture_ReturnsTeaCmd(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	req := k8s.CaptureRequest{
		Backend: k8s.BackendKubectlDebug, Context: "ctx", Namespace: "ns", PodName: "pod1",
		Interface: "any", SnapLen: 65535, OutputDir: t.TempDir(),
	}
	cmd := m.startCapture(req, newCaptureRing(50))
	if cmd == nil {
		t.Fatal("startCapture returned nil cmd")
	}
	// Don't call cmd() — backend factory will try to spawn kubectl debug.
	// Just verifying the closure constructs.
}

func TestStopCapture_ReturnsTeaCmd(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	cmd := m.stopCapture(99) // ID doesn't exist; Stop returns "not found"
	if cmd == nil {
		t.Fatal("stopCapture returned nil cmd")
	}
	msg := cmd()
	if _, ok := msg.(captureStoppedMsg); !ok {
		t.Errorf("got msg type %T, want captureStoppedMsg", msg)
	}
}

func TestLaunchKubeshark_ReturnsTeaCmd(t *testing.T) {
	m := baseFinalModel()
	cmd := m.launchKubeshark(model.Item{Name: "pod1", Namespace: "ns"})
	if cmd == nil {
		t.Fatal("launchKubeshark returned nil cmd")
	}
	// Don't call cmd() — would try to actually port-forward + open browser.
}

func TestWaitForCaptureUpdate_StaysBlockedWithoutAStateChange(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()

	cmd := m.waitForCaptureUpdate()
	if cmd == nil {
		t.Fatal("waitForCaptureUpdate returned nil cmd")
	}

	got := make(chan tea.Msg, 1)
	go func() { got <- cmd() }()

	// StopAll has no entry to transition, so nothing should be reported.
	m.captureMgr.StopAll()

	select {
	case msg := <-got:
		t.Errorf("waitForCaptureUpdate returned spurious msg: %T", msg)
	case <-time.After(20 * time.Millisecond):
	}

	// Release the waiter so it does not outlive the test.
	m.captureMgr.SetUpdateListener(make(chan struct{}, 1))
	select {
	case msg := <-got:
		if msg != nil {
			t.Errorf("superseded waiter reported an update it never received: %T", msg)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("superseded waitForCaptureUpdate listener did not exit")
	}
}

// SetUpdateListener keeps only one slot, so an older listener must be
// released rather than blocking forever once a newer one is armed.
func TestWaitForCaptureUpdate_SupersededListenerExits(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()

	firstCmd := m.waitForCaptureUpdate()
	if firstCmd == nil {
		t.Fatal("waitForCaptureUpdate returned nil cmd")
	}
	firstDone := make(chan tea.Msg, 1)
	go func() { firstDone <- firstCmd() }()

	// Give the first goroutine time to reach its select before it is superseded.
	time.Sleep(20 * time.Millisecond)

	if second := m.waitForCaptureUpdate(); second == nil {
		t.Fatal("waitForCaptureUpdate returned nil cmd on re-arm")
	}

	select {
	case <-firstDone:
	case <-time.After(2 * time.Second):
		t.Fatal("superseded waitForCaptureUpdate listener did not exit when a new listener was armed")
	}
}
