package app

import (
	"testing"

	"github.com/janosmiko/lfk/internal/k8s"
	"github.com/janosmiko/lfk/internal/model"
)

func TestCapturesPseudoItems_NilManager_ReturnsEmpty(t *testing.T) {
	got := capturesPseudoItems(nil)
	if len(got) != 0 {
		t.Errorf("nil mgr should return empty; got %d items", len(got))
	}
}

func TestCapturesPseudoItems_EmptyManager_ReturnsEmpty(t *testing.T) {
	mgr := k8s.NewCaptureManager()
	got := capturesPseudoItems(mgr)
	if len(got) != 0 {
		t.Errorf("empty mgr should return 0 items; got %d", len(got))
	}
}

// --- action handler tests ---

func TestStopCaptureFromPseudo_DispatchesStopCmd(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	// ID 42 doesn't exist in the manager; stopCapture still returns a non-nil cmd.
	it := model.Item{Kind: "__captures__", Extra: "42"}

	_, cmd := m.stopCaptureFromPseudo(it)
	if cmd == nil {
		t.Fatal("stopCaptureFromPseudo returned nil cmd")
	}
}

func TestStopCaptureFromPseudo_InvalidID(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	it := model.Item{Kind: "__captures__", Extra: "not-a-number"}

	out, _ := m.stopCaptureFromPseudo(it)
	// Should NOT panic; just returns a status message.
	_ = out.(Model)
}

func TestOpenCaptureFromPseudo_NotFound(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	it := model.Item{Kind: "__captures__", Extra: "999"}

	out, _ := m.openCaptureFromPseudo(it)
	mm := out.(Model)
	if mm.overlay == overlayTrafficCapture {
		t.Errorf("openCaptureFromPseudo should not open overlay for missing ID")
	}
}

func TestDeleteCaptureFile_InvalidID(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	it := model.Item{Kind: "__captures__", Extra: "bad"}

	out, _ := m.deleteCaptureFile(it)
	_ = out.(Model) // should not panic
}

func TestDeleteCaptureFile_NotFound(t *testing.T) {
	m := baseFinalModel()
	m.captureMgr = k8s.NewCaptureManager()
	it := model.Item{Kind: "__captures__", Extra: "7"}

	out, _ := m.deleteCaptureFile(it)
	_ = out.(Model) // capture not found — should not panic
}

func TestGetCaptureIDFromItem_Valid(t *testing.T) {
	it := model.Item{Extra: "42"}
	id, ok := getCaptureIDFromItem(it)
	if !ok || id != 42 {
		t.Errorf("expected id=42, ok=true; got id=%d ok=%v", id, ok)
	}
}

func TestGetCaptureIDFromItem_Invalid(t *testing.T) {
	it := model.Item{Extra: "abc"}
	_, ok := getCaptureIDFromItem(it)
	if ok {
		t.Error("expected ok=false for non-numeric Extra")
	}
}

func TestGetCaptureIDFromItem_Zero(t *testing.T) {
	it := model.Item{Extra: "0"}
	_, ok := getCaptureIDFromItem(it)
	if ok {
		t.Error("expected ok=false for Extra=0 (IDs must be positive)")
	}
}
